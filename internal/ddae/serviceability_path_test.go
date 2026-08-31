package ddae

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestServiceabilityDetailUsesOneEncodedRequestSegment(t *testing.T) {
	id := "日誌/one segment"
	var requestURI string
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == tokenPath {
			fmt.Fprint(writer, `{"access_token":"token","expires_in":3600}`)
			return
		}
		requestURI = request.RequestURI
		fmt.Fprintf(writer, `{"id":%q}`, id)
	}))
	defer server.Close()
	client, err := NewClient(clientConfig(t, server.URL, trustedServerCA(t, server), map[string]string{
		"DDAE_API_PATH_PREFIX": "/custom-api",
	}))
	if err != nil {
		t.Fatal(err)
	}
	defer client.CloseIdleConnections()
	result, err := client.ServiceabilityLogDetail(context.Background(), id)
	if err != nil || result.ID != id {
		t.Fatalf("detail=%#v err=%v", result, err)
	}
	want := "/custom-api/serviceability-events/%E6%97%A5%E8%AA%8C%2Fone%20segment"
	if requestURI != want {
		t.Fatalf("request URI = %q, want %q", requestURI, want)
	}
}
