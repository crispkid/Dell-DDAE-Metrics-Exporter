package ddae

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/config"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/observability"
)

type tokenManager struct {
	mu             sync.Mutex
	baseURL        *url.URL
	httpClient     *http.Client
	requestTimeout time.Duration
	username       config.Secret
	password       config.Secret
	clientSecret   config.Secret
	token          string
	refreshAt      time.Time
	generation     uint64
	retryMax       int
	inflight       *tokenRefresh
}

type tokenLease struct {
	token      string
	generation uint64
}

type tokenRefresh struct {
	done   chan struct{}
	result tokenRefreshResult
}

type tokenRefreshResult struct {
	lease     tokenLease
	refreshAt time.Time
	err       error
}

func (m *tokenManager) get(ctx context.Context, rejectedGeneration uint64) (tokenLease, error) {
	m.mu.Lock()
	now := time.Now()
	usable := m.token != "" && (m.refreshAt.IsZero() || now.Before(m.refreshAt))
	if usable && (rejectedGeneration == 0 || m.generation > rejectedGeneration) {
		lease := tokenLease{token: m.token, generation: m.generation}
		m.mu.Unlock()
		return lease, nil
	}
	if m.inflight != nil {
		refresh := m.inflight
		m.mu.Unlock()
		select {
		case <-ctx.Done():
			return tokenLease{}, &Error{class: observability.ClassTimeout, op: "token"}
		case <-refresh.done:
			return refresh.result.lease, refresh.result.err
		}
	}
	refresh := &tokenRefresh{done: make(chan struct{})}
	m.inflight = refresh
	nextGeneration := m.generation + 1
	m.mu.Unlock()

	token, refreshAt, err := m.requestWithRetry(ctx)
	result := tokenRefreshResult{lease: tokenLease{token: token, generation: nextGeneration}, refreshAt: refreshAt, err: err}
	m.mu.Lock()
	if err == nil {
		m.token = token
		m.refreshAt = refreshAt
		m.generation = nextGeneration
	}
	refresh.result = result
	if m.inflight == refresh {
		m.inflight = nil
	}
	close(refresh.done)
	m.mu.Unlock()
	return result.lease, result.err
}

func (m *tokenManager) requestWithRetry(ctx context.Context) (string, time.Time, error) {
	for attempt := 0; attempt <= m.retryMax; attempt++ {
		token, refreshAt, status, err := m.request(ctx)
		if err == nil {
			return token, refreshAt, nil
		}
		if !tokenRetryable(status, err) || attempt == m.retryMax {
			return "", time.Time{}, err
		}
		if err := waitBackoff(ctx, attempt); err != nil {
			return "", time.Time{}, &Error{class: observability.ClassTimeout, op: "token"}
		}
	}
	return "", time.Time{}, &Error{class: observability.ClassInternal, op: "token"}
}

func (m *tokenManager) request(ctx context.Context) (string, time.Time, int, error) {
	requestContext, cancel := context.WithTimeout(ctx, m.requestTimeout)
	defer cancel()
	values := url.Values{
		"grant_type":    {"password"},
		"client_id":     {"dv-admin-rest"},
		"client_secret": {m.clientSecret.Value()},
		"username":      {m.username.Value()},
		"password":      {m.password.Value()},
	}
	target := cloneURL(m.baseURL)
	target.Path = tokenPath
	request, err := http.NewRequestWithContext(requestContext, http.MethodPost, target.String(), strings.NewReader(values.Encode()))
	if err != nil {
		return "", time.Time{}, 0, &Error{class: observability.ClassInternal, op: "token"}
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")
	response, err := m.httpClient.Do(request)
	if err != nil {
		class := transportFailureClass(err, requestContext)
		return "", time.Time{}, 0, &Error{class: class, op: "token"}
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.CopyN(io.Discard, response.Body, 4096)
		class := observability.ClassHTTP
		if response.StatusCode == http.StatusBadRequest || response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
			class = observability.ClassAuth
		}
		return "", time.Time{}, response.StatusCode, &Error{class: class, op: "token", status: response.StatusCode}
	}
	var result struct {
		AccessToken string      `json:"access_token"`
		ExpiresIn   json.Number `json:"expires_in"`
	}
	if err := decodeBounded(response.Body, tokenBodyLimit, &result); err != nil || result.AccessToken == "" || strings.ContainsAny(result.AccessToken, "\r\n\x00") {
		return "", time.Time{}, response.StatusCode, &Error{class: observability.ClassDecode, op: "token"}
	}
	var refreshAt time.Time
	if result.ExpiresIn != "" {
		seconds, err := strconv.ParseInt(string(result.ExpiresIn), 10, 64)
		if err == nil && seconds > 0 {
			now := time.Now()
			lifetime := time.Duration(seconds) * time.Second
			refreshAt = now.Add(lifetime * 8 / 10)
			beforeExpiry := now.Add(lifetime - time.Minute)
			if beforeExpiry.After(now) && beforeExpiry.Before(refreshAt) {
				refreshAt = beforeExpiry
			}
		}
	}
	return result.AccessToken, refreshAt, response.StatusCode, nil
}

func tokenRetryable(status int, err error) bool {
	if status == http.StatusTooManyRequests || status >= 500 && status <= 599 {
		return true
	}
	if status != 0 {
		return false
	}
	var classified observability.Classified
	if !errors.As(err, &classified) {
		return false
	}
	class := classified.FailureClass()
	return class == observability.ClassTimeout || class == observability.ClassTransport
}
