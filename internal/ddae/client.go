package ddae

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/config"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/observability"
)

const (
	tokenBodyLimit         = 64 * 1024
	maxResponseBodyBytes   = 64 * 1024 * 1024
	maxResponseHeaderBytes = 1024 * 1024
)

type Error struct {
	class  observability.Class
	op     string
	status int
}

func (e *Error) Error() string {
	if e.status != 0 {
		return fmt.Sprintf("%s failed with HTTP status %d", e.op, e.status)
	}
	return e.op + " failed"
}

func (e *Error) FailureClass() observability.Class { return e.class }

type Client struct {
	baseURL                      *url.URL
	httpClient                   *http.Client
	tokens                       *tokenManager
	requestTimeout               time.Duration
	responseLimit                int64
	listLimit                    int64
	detailLimit                  int64
	serviceabilityLogListLimit   int64
	serviceabilityLogDetailLimit int64
	retryMax                     int
}

func NewClient(cfg config.Config) (*Client, error) {
	if cfg.DDAETLSInsecureSkipVerify && !cfg.AllowInsecureTLS {
		return nil, errors.New("DDAE insecure TLS requires the global acknowledgement")
	}
	if cfg.DDAETLSInsecureSkipVerify && cfg.DDAECAFile != "" {
		return nil, errors.New("DDAE custom CA conflicts with insecure TLS")
	}
	tlsConfig, err := tlsConfig(cfg.DDAECAFile, cfg.DDAETLSInsecureSkipVerify)
	if err != nil {
		return nil, err
	}
	transport := &http.Transport{
		Proxy:                  nil,
		TLSClientConfig:        tlsConfig,
		ForceAttemptHTTP2:      true,
		MaxIdleConns:           32,
		MaxIdleConnsPerHost:    16,
		IdleConnTimeout:        90 * time.Second,
		TLSHandshakeTimeout:    cfg.RequestTimeout,
		ResponseHeaderTimeout:  cfg.RequestTimeout,
		ExpectContinueTimeout:  time.Second,
		MaxResponseHeaderBytes: maxResponseHeaderBytes,
	}
	httpClient := &http.Client{
		Transport: transport,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return errors.New("redirects are disabled")
		},
	}
	tokens := &tokenManager{
		baseURL:        cloneURL(cfg.DDAEBaseURL),
		httpClient:     httpClient,
		requestTimeout: cfg.RequestTimeout,
		username:       cfg.DDAEUsername,
		password:       cfg.DDAEPassword,
		clientSecret:   cfg.DDAEClientSecret,
		retryMax:       cfg.RetryMax,
	}
	return &Client{
		baseURL:                      cloneURL(cfg.DDAEBaseURL),
		httpClient:                   httpClient,
		tokens:                       tokens,
		requestTimeout:               cfg.RequestTimeout,
		responseLimit:                cfg.ResponseMaxBytes,
		listLimit:                    cfg.AlertListResponseMaxBytes,
		detailLimit:                  cfg.AlertDetailResponseMaxBytes,
		serviceabilityLogListLimit:   cfg.ServiceabilityLogListResponseMaxBytes,
		serviceabilityLogDetailLimit: cfg.ServiceabilityLogDetailResponseMaxBytes,
		retryMax:                     cfg.RetryMax,
	}, nil
}

func tlsConfig(caFile string, insecureSkipVerify ...bool) (*tls.Config, error) {
	rootCAs, err := x509.SystemCertPool()
	if err != nil || rootCAs == nil {
		rootCAs = x509.NewCertPool()
	}
	if caFile != "" {
		pem, err := os.ReadFile(caFile)
		if err != nil {
			return nil, errors.New("cannot read DDAE_CA_FILE")
		}
		if ok := rootCAs.AppendCertsFromPEM(pem); !ok {
			return nil, errors.New("DDAE_CA_FILE contains no usable certificate")
		}
	}
	skipVerification := len(insecureSkipVerify) > 0 && insecureSkipVerify[0]
	return &tls.Config{
		MinVersion:         config.TLSMinVersion(),
		RootCAs:            rootCAs,
		InsecureSkipVerify: skipVerification, // #nosec G402 -- guarded by the approved two-level configuration policy.
	}, nil
}

func (c *Client) CloseIdleConnections() { c.httpClient.CloseIdleConnections() }

func (c *Client) Ping(ctx context.Context) (PingResponse, error) {
	var result PingResponse
	err := c.getJSON(ctx, "ping", pingPath, c.responseLimit, &result)
	if err == nil && strings.TrimSpace(result.Status) == "" {
		err = &Error{class: observability.ClassValidation, op: "ping"}
	}
	return result, err
}

func (c *Client) Clusters(ctx context.Context) ([]Cluster, error) {
	var result []Cluster
	err := c.getJSON(ctx, "clusters", clustersPath, c.responseLimit, &result)
	return result, err
}

func (c *Client) Nodes(ctx context.Context) ([]InfrastructureNode, error) {
	var result []InfrastructureNode
	err := c.getJSON(ctx, "nodes", nodesPath, c.responseLimit, &result)
	return result, err
}

func (c *Client) Lock(ctx context.Context) (LockResponse, error) {
	var result LockResponse
	err := c.getJSON(ctx, "lock", lockPath, c.responseLimit, &result)
	if err == nil {
		if _, ok := result.Status.Value(); !ok {
			err = &Error{class: observability.ClassValidation, op: "lock"}
		}
	}
	return result, err
}

func (c *Client) Power(ctx context.Context) (PowerResponse, error) {
	var result PowerResponse
	err := c.getJSON(ctx, "power", powerPath, c.responseLimit, &result)
	return result, err
}

func (c *Client) AlertList(ctx context.Context) (AlertList, error) {
	var result AlertList
	err := c.getJSON(ctx, "alert_list", alertListPath, c.listLimit, &result)
	return result, err
}

func (c *Client) AlertDetail(ctx context.Context, id string) (AlertDetail, error) {
	if err := ValidateAlertID(id); err != nil {
		return AlertDetail{}, err
	}
	var result AlertDetail
	err := c.getJSON(ctx, "alert_detail", alertDetailPath+url.PathEscape(id), c.detailLimit, &result)
	return result, err
}

func (c *Client) ServiceabilityLogList(ctx context.Context) (ServiceabilityLogList, error) {
	var result ServiceabilityLogList
	err := c.getJSON(ctx, "serviceability_log_list", serviceabilityLogListPath, c.serviceabilityLogListLimit, &result)
	return result, err
}

func (c *Client) ServiceabilityLogDetail(ctx context.Context, id string) (ServiceabilityLogDetail, error) {
	if err := ValidateServiceabilityLogID(id); err != nil {
		return ServiceabilityLogDetail{}, err
	}
	var result ServiceabilityLogDetail
	err := c.getJSONRawPath(ctx, "serviceability_log_detail", serviceabilityLogDetailPath+id,
		serviceabilityLogDetailPath+url.PathEscape(id), c.serviceabilityLogDetailLimit, &result)
	return result, err
}

func (c *Client) getJSON(ctx context.Context, operation, path string, limit int64, destination any) error {
	return c.getJSONRawPath(ctx, operation, path, "", limit, destination)
}

func (c *Client) getJSONRawPath(ctx context.Context, operation, path, rawPath string, limit int64, destination any) error {
	lease, err := c.tokens.get(ctx, 0)
	if err != nil {
		return err
	}
	authRetried := false
	for attempt := 0; attempt <= c.retryMax; attempt++ {
		status, err := c.doGET(ctx, operation, path, rawPath, lease.token, limit, destination)
		if status == http.StatusUnauthorized {
			if authRetried {
				return &Error{class: observability.ClassAuth, op: operation, status: status}
			}
			lease, err = c.tokens.get(ctx, lease.generation)
			if err != nil {
				return err
			}
			authRetried = true
			attempt--
			continue
		}
		if err == nil {
			return nil
		}
		if !retryable(status, err) || attempt == c.retryMax {
			return err
		}
		if err := waitBackoff(ctx, attempt); err != nil {
			return &Error{class: observability.ClassTimeout, op: operation}
		}
	}
	return &Error{class: observability.ClassInternal, op: operation}
}

func (c *Client) doGET(ctx context.Context, operation, path, rawPath, token string, limit int64, destination any) (int, error) {
	requestContext, cancel := context.WithTimeout(ctx, c.requestTimeout)
	defer cancel()
	target := cloneURL(c.baseURL)
	target.Path = path
	target.RawPath = rawPath
	request, err := http.NewRequestWithContext(requestContext, http.MethodGet, target.String(), nil)
	if err != nil {
		return 0, &Error{class: observability.ClassInternal, op: operation}
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := c.httpClient.Do(request)
	if err != nil {
		class := transportFailureClass(err, requestContext)
		return 0, &Error{class: class, op: operation}
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.CopyN(io.Discard, response.Body, 4096)
		class := observability.ClassHTTP
		if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
			class = observability.ClassAuth
		}
		return response.StatusCode, &Error{class: class, op: operation, status: response.StatusCode}
	}
	if err := decodeBounded(response.Body, limit, destination); err != nil {
		return response.StatusCode, &Error{class: observability.ClassDecode, op: operation}
	}
	return response.StatusCode, nil
}

func decodeBounded(reader io.Reader, limit int64, destination any) error {
	if limit < 1 || limit > maxResponseBodyBytes {
		return errors.New("invalid response limit")
	}
	data, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return err
	}
	if int64(len(data)) > limit {
		return errors.New("response exceeds limit")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return errors.New("response contains trailing JSON")
	}
	return nil
}

func retryable(status int, err error) bool {
	if err == nil {
		return false
	}
	if status == 0 {
		var classified observability.Classified
		if errors.As(err, &classified) {
			class := classified.FailureClass()
			return class == observability.ClassTimeout || class == observability.ClassTransport
		}
	}
	switch status {
	case http.StatusTooManyRequests, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func waitBackoff(ctx context.Context, attempt int) error {
	base := 100 * time.Millisecond * time.Duration(1<<min(attempt, 5))
	jitter := time.Duration(rand.IntN(100)) * time.Millisecond
	timer := time.NewTimer(base + jitter)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func ValidateAlertID(id string) error {
	if len(id) < 1 || len(id) > 256 || id == "." || id == ".." {
		return &Error{class: observability.ClassValidation, op: "alert_detail_id"}
	}
	for i := 0; i < len(id); i++ {
		ch := id[i]
		valid := ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' || strings.ContainsRune("._:-", rune(ch))
		if !valid {
			return &Error{class: observability.ClassValidation, op: "alert_detail_id"}
		}
	}
	return nil
}

func ValidateServiceabilityLogID(id string) error {
	if len(id) < 1 || len(id) > 256 || !utf8.ValidString(id) || id == "." || id == ".." || strings.ContainsRune(id, '\x00') {
		return &Error{class: observability.ClassValidation, op: "serviceability_log_detail_id"}
	}
	return nil
}

func cloneURL(source *url.URL) *url.URL {
	copy := *source
	return &copy
}

func transportFailureClass(err error, ctx context.Context) observability.Class {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return observability.ClassTimeout
	}
	var unknownAuthority x509.UnknownAuthorityError
	var hostname x509.HostnameError
	var certificateInvalid x509.CertificateInvalidError
	var recordHeader tls.RecordHeaderError
	if errors.As(err, &unknownAuthority) || errors.As(err, &hostname) || errors.As(err, &certificateInvalid) || errors.As(err, &recordHeader) {
		return observability.ClassTLS
	}
	return observability.ClassTransport
}
