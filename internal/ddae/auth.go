package ddae

import (
	"context"
	"encoding/json"
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
	refreshing     bool
	done           chan struct{}
}

func (m *tokenManager) get(ctx context.Context, force bool) (string, error) {
	m.mu.Lock()
	initialGeneration := m.generation
	for {
		now := time.Now()
		usable := m.token != "" && (m.refreshAt.IsZero() || now.Before(m.refreshAt))
		if usable && (!force || m.generation > initialGeneration) {
			token := m.token
			m.mu.Unlock()
			return token, nil
		}
		if !m.refreshing {
			m.refreshing = true
			m.done = make(chan struct{})
			done := m.done
			m.mu.Unlock()
			token, refreshAt, err := m.request(ctx)
			m.mu.Lock()
			if err == nil {
				m.token = token
				m.refreshAt = refreshAt
				m.generation++
			}
			m.refreshing = false
			close(done)
			m.mu.Unlock()
			if err != nil {
				return "", err
			}
			return token, nil
		}
		done := m.done
		m.mu.Unlock()
		select {
		case <-ctx.Done():
			return "", &Error{class: observability.ClassTimeout, op: "token"}
		case <-done:
		}
		m.mu.Lock()
	}
}

func (m *tokenManager) request(ctx context.Context) (string, time.Time, error) {
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
		return "", time.Time{}, &Error{class: observability.ClassInternal, op: "token"}
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")
	response, err := m.httpClient.Do(request)
	if err != nil {
		class := transportFailureClass(err, requestContext)
		return "", time.Time{}, &Error{class: class, op: "token"}
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.CopyN(io.Discard, response.Body, 4096)
		return "", time.Time{}, &Error{class: observability.ClassAuth, op: "token", status: response.StatusCode}
	}
	var result struct {
		AccessToken string      `json:"access_token"`
		ExpiresIn   json.Number `json:"expires_in"`
	}
	if err := decodeBounded(response.Body, tokenBodyLimit, &result); err != nil || result.AccessToken == "" || strings.ContainsAny(result.AccessToken, "\r\n\x00") {
		return "", time.Time{}, &Error{class: observability.ClassDecode, op: "token"}
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
	return result.AccessToken, refreshAt, nil
}
