package ddae

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"time"
)

type recordedTransport struct{ body []byte }

func (t recordedTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.Method != http.MethodGet {
		return nil, errors.New("offline network prohibited")
	}
	return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(t.body)), Request: r}, nil
}

// decodeClusterResponse cannot dial a socket: even its HTTP transport is an in-memory
// reader. It exercises the production methods, including post-decode checks.
func decodeClusterResponse(body []byte) (any, error) {
	routes, _ := routeSetForPrefixes("", "/v1")
	origin := &url.URL{Scheme: "https", Host: "replay.example.invalid"}
	c := &Client{baseURL: origin, routes: routes, httpClient: &http.Client{Transport: recordedTransport{body}},
		tokens: &tokenManager{token: "offline-synthetic"}, requestTimeout: time.Second,
		responseLimit: maxResponseBodyBytes, listLimit: maxResponseBodyBytes, detailLimit: maxResponseBodyBytes,
		serviceabilityLogListLimit: maxResponseBodyBytes, serviceabilityLogDetailLimit: maxResponseBodyBytes}
	return c.Clusters(context.Background())
}
