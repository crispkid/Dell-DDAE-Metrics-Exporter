package ddae

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/config"
)

// DiagnosticExchange is confidential. Only the separately approved encrypted
// diagnostic sink may serialize it. Token exchanges deliberately contain none
// of the request URL, header values or body.
type DiagnosticExchange struct {
	ID              int         `json:"request_id"`
	Operation       string      `json:"operation"`
	RequestedID     string      `json:"requested_id,omitempty"`
	Started         string      `json:"started_utc"`
	Milliseconds    int64       `json:"duration_ms"`
	Method          string      `json:"method,omitempty"`
	URL             string      `json:"url,omitempty"`
	Protocol        string      `json:"http_version,omitempty"`
	RequestHeaders  http.Header `json:"request_headers,omitempty"`
	RequestBody     []byte      `json:"request_body_base64"`
	ResponseHeaders http.Header `json:"response_headers,omitempty"`
	Status          int         `json:"http_status"`
	Body            []byte      `json:"body_base64,omitempty"`
	ContentDecoded  bool        `json:"content_decoded"`
	Complete        bool        `json:"capture_complete"`
	Reason          string      `json:"reason,omitempty"`
}
type DiagnosticObserver interface {
	Begin(string) (int, error)
	Observe(DiagnosticExchange) error
}
type diagnosticTransport struct {
	next       http.RoundTripper
	observer   DiagnosticObserver
	routes     []Operation
	limits     map[string]int64
	secrets    []string
	decodeGzip bool
}

// NewDiagnosticClient is not used by the normal exporter entry point/config.
// It retains the same origin, TLS, methods, retries, token manager and parsers.
func NewDiagnosticClient(cfg config.Config, observer DiagnosticObserver, capBytes int64) (*Client, error) {
	if observer == nil || capBytes < 1 || capBytes > maxResponseBodyBytes {
		return nil, errors.New("invalid diagnostic observer")
	}
	c, err := NewClient(cfg)
	if err != nil {
		return nil, err
	}
	// Decode in the observer so captured headers retain the original encoding
	// and content length. The normal exporter transport is untouched.
	transport := c.httpClient.Transport.(*http.Transport)
	transport.DisableCompression = true
	c.httpClient.Transport = &diagnosticTransport{
		next: c.httpClient.Transport, observer: observer, routes: c.routes.operations(),
		decodeGzip: true,
		secrets:    []string{cfg.DDAEUsername.Value(), cfg.DDAEPassword.Value(), cfg.DDAEClientSecret.Value()},
		limits: map[string]int64{
			"ping": min(capBytes, c.responseLimit), "clusters": min(capBytes, c.responseLimit), "nodes": min(capBytes, c.responseLimit),
			"lock": min(capBytes, c.responseLimit), "power": min(capBytes, c.responseLimit),
			"alert_list": min(capBytes, c.listLimit), "alert_detail": min(capBytes, c.detailLimit),
			"serviceability_log_list": min(capBytes, c.serviceabilityLogListLimit), "serviceability_log_detail": min(capBytes, c.serviceabilityLogDetailLimit),
		},
	}
	return c, nil
}
func (t *diagnosticTransport) CloseIdleConnections() {
	if closer, ok := t.next.(interface{ CloseIdleConnections() }); ok {
		closer.CloseIdleConnections()
	}
}
func (t *diagnosticTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	op, id := "", ""
	if req.URL.Path == tokenPath {
		op = "token"
	} else if req.Method == http.MethodGet {
		for _, route := range t.routes {
			if strings.HasSuffix(route.Path, "{id}") {
				prefix := strings.TrimSuffix(route.Path, "{id}")
				if strings.HasPrefix(req.URL.Path, prefix) {
					op = route.Collector
					id = strings.TrimPrefix(req.URL.Path, prefix)
					break
				}
			} else if req.URL.Path == route.Path {
				op = route.Collector
				break
			}
		}
	}
	if op == "" {
		return nil, errors.New("unapproved diagnostic operation")
	}
	number, err := t.observer.Begin(op)
	if err != nil {
		return nil, err
	}
	started := time.Now()
	e := DiagnosticExchange{ID: number, Operation: op, Started: started.UTC().Format(time.RFC3339Nano)}
	requestedGzip := t.decodeGzip && req.Header.Get("Accept-Encoding") == "" && req.Header.Get("Range") == "" && req.Method != http.MethodHead
	if requestedGzip {
		req = req.Clone(req.Context())
		req.Header.Set("Accept-Encoding", "gzip")
	}
	response, transportErr := t.next.RoundTrip(req)
	if response != nil {
		e.Status = response.StatusCode
	}
	var originalHeaders http.Header
	if response != nil {
		originalHeaders = response.Header.Clone()
		if requestedGzip && response.Header.Get("Content-Encoding") == "gzip" {
			originalBody := response.Body
			reader, err := gzip.NewReader(originalBody)
			if err != nil {
				response.Body = &diagnosticGzipBody{Reader: diagnosticReadError{}, original: originalBody}
			} else {
				response.Body = &diagnosticGzipBody{Reader: reader, original: originalBody, gzip: reader}
			}
			response.Uncompressed = true
			response.ContentLength = -1
			response.Header.Del("Content-Encoding")
			response.Header.Del("Content-Length")
		}
	}
	if op == "token" {
		e.Milliseconds = time.Since(started).Milliseconds()
		e.Complete = false
		e.Reason = "authentication_content_excluded"
		if err := t.observer.Observe(e); err != nil {
			if response != nil {
				response.Body.Close()
			}
			return nil, err
		}
		return response, transportErr
	}
	secrets := append([]string{}, t.secrets...)
	if token := req.Header.Get("Authorization"); token != "" {
		secrets = append(secrets, token, strings.TrimPrefix(token, "Bearer "))
	}
	e.RequestedID = id
	e.Method = req.Method
	e.RequestBody = []byte{}
	e.URL = req.URL.String()
	e.RequestHeaders = diagnosticHeaders(req.Header, secrets)
	if response == nil {
		e.Reason = "no_response"
	} else {
		e.Protocol = response.Proto
		e.ContentDecoded = response.Uncompressed
		e.ResponseHeaders = diagnosticHeaders(originalHeaders, secrets)
		limit := t.limits[op]
		body, readErr := io.ReadAll(io.LimitReader(response.Body, limit+1))
		response.Body.Close()
		if int64(len(body)) > limit {
			body = body[:limit]
			readErr = errors.New("diagnostic body limit")
			e.Reason = "body_limit"
		}
		if readErr != nil && e.Reason == "" {
			e.Reason = "body_read_error"
		}
		e.Body = body
		e.Complete = readErr == nil && transportErr == nil
		// Restore exactly what was read, including a read failure, for the production
		// decoder. A captured prefix must never become a successful full document.
		var reader io.Reader = bytes.NewReader(body)
		if readErr != nil {
			reader = io.MultiReader(reader, diagnosticReadError{})
		}
		response.Body = io.NopCloser(reader)
	}
	e.Milliseconds = time.Since(started).Milliseconds()
	if err := t.observer.Observe(e); err != nil {
		if response != nil {
			response.Body.Close()
		}
		return nil, err
	}
	return response, transportErr
}

type diagnosticReadError struct{}

type diagnosticGzipBody struct {
	io.Reader
	original io.Closer
	gzip     *gzip.Reader
}

func (b *diagnosticGzipBody) Close() error {
	if b.gzip != nil {
		_ = b.gzip.Close()
	}
	return b.original.Close()
}

func (diagnosticReadError) Read([]byte) (int, error) {
	return 0, errors.New("incomplete diagnostic body")
}
func diagnosticHeaders(source http.Header, secrets []string) http.Header {
	result := http.Header{}
	for k, values := range source {
		switch strings.ToLower(k) {
		case "authorization", "proxy-authorization", "cookie", "set-cookie":
			continue
		}
		safe := true
		for _, v := range append([]string{k}, values...) {
			for _, secret := range secrets {
				if secret != "" && strings.Contains(v, secret) {
					safe = false
				}
			}
		}
		if safe {
			result[k] = append([]string{}, values...)
		}
	}
	return result
}

func (c *Client) Inspect(ctx context.Context, op, id string) (any, error) {
	switch op {
	case "ping":
		return c.Ping(ctx)
	case "clusters":
		return c.Clusters(ctx)
	case "nodes":
		return c.Nodes(ctx)
	case "lock":
		return c.Lock(ctx)
	case "power":
		return c.Power(ctx)
	case "alert_list":
		return c.AlertList(ctx)
	case "alert_detail":
		return c.AlertDetail(ctx, id)
	case "serviceability_log_list":
		return c.ServiceabilityLogList(ctx)
	case "serviceability_log_detail":
		return c.ServiceabilityLogDetail(ctx, id)
	default:
		return nil, errors.New("unsupported diagnostic operation")
	}
}

type recordedTransport struct{ body []byte }

func (t recordedTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.Method != http.MethodGet {
		return nil, errors.New("offline network prohibited")
	}
	return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(t.body)), Request: r}, nil
}

// DecodeRecorded cannot dial a socket: even its HTTP transport is an in-memory
// reader. It exercises the production methods, including post-decode checks.
func DecodeRecorded(op, id string, body []byte) (any, error) {
	routes, _ := routeSetForPrefixes("", "/v1")
	origin := &url.URL{Scheme: "https", Host: "replay.example.invalid"}
	c := &Client{baseURL: origin, routes: routes, httpClient: &http.Client{Transport: recordedTransport{body}},
		tokens: &tokenManager{token: "offline-synthetic"}, requestTimeout: time.Second,
		responseLimit: maxResponseBodyBytes, listLimit: maxResponseBodyBytes, detailLimit: maxResponseBodyBytes,
		serviceabilityLogListLimit: maxResponseBodyBytes, serviceabilityLogDetailLimit: maxResponseBodyBytes}
	return c.Inspect(context.Background(), op, id)
}
