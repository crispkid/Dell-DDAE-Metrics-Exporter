package queryclient

import (
	"context"
	"encoding/pem"
	"fmt"
	"github.com/crispkid/dell-ddae-metrics-exporter/internal/config"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func TestQueryRedirectBoundary(t *testing.T) {
	engine, _ := url.Parse("https://engine.invalid")
	auth, _ := url.Parse("https://auth.invalid")
	c := &Client{base: engine, auth: auth, realm: "ddae"}
	for _, s := range []string{"http://auth.invalid/auth/realms/ddae/protocol/openid-connect/auth", "https://evil.invalid/auth/realms/ddae/protocol/openid-connect/auth", "https://engine.invalid/ui/api/query/q/killed", "https://auth.invalid/auth/realms/other/protocol/openid-connect/auth"} {
		u, _ := url.Parse(s)
		if c.allowedLogin(u) {
			t.Fatal("unsafe redirect allowed")
		}
	}
	for _, s := range []string{"https://engine.invalid/oauth2/callback?code=synthetic", "https://auth.invalid/auth/realms/ddae/protocol/openid-connect/auth"} {
		u, _ := url.Parse(s)
		if !c.allowedLogin(u) {
			t.Fatal("valid redirect rejected")
		}
	}
}

func TestQueryOIDCSessionAndConcurrentRenewal(t *testing.T) {
	var logins atomic.Int64
	var engine, identity *httptest.Server
	identity = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth/realms/ddae/protocol/openid-connect/auth":
			fmt.Fprintf(w, `<form id="kc-form-login" method="post" action="%s/auth/realms/ddae/login-actions/authenticate"><input name="username"><input name="password" type="password"><input name="login"></form>`, identity.URL)
		case "/auth/realms/ddae/login-actions/authenticate":
			if r.Method != "POST" {
				t.Error("unexpected method")
			}
			r.ParseForm()
			if r.Form.Get("username") != "synthetic" || r.Form.Get("password") != "synthetic" {
				t.Error("wrong login")
			}
			logins.Add(1)
			http.Redirect(w, r, engine.URL+"/oauth2/callback", 303)
		default:
			http.NotFound(w, r)
		}
	}))
	defer identity.Close()
	engine = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ui/insights/login":
			http.Redirect(w, r, "/ui/insights/login/proceed", 303)
		case "/ui/insights/login/proceed":
			http.Redirect(w, r, identity.URL+"/auth/realms/ddae/protocol/openid-connect/auth", 303)
		case "/oauth2/callback":
			http.SetCookie(w, &http.Cookie{Name: "session", Value: "synthetic", Path: "/", Secure: true, HttpOnly: true})
			http.Redirect(w, r, "/ui/insights/", 303)
		case "/ui/insights/":
			fmt.Fprint(w, "<html>Insights</html>")
		case "/ui/api/insights/cluster/info":
			if _, err := r.Cookie("session"); err != nil {
				w.WriteHeader(401)
				return
			}
			if r.Header.Get("X-Trino-Role") != "system=ROLE{sysadmin}" {
				t.Error("role missing")
			}
			fmt.Fprint(w, `{"allQueries":true}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer engine.Close()
	t.Setenv("QUERY_ENABLED", "true")
	t.Setenv("QUERY_BASE_URL", engine.URL)
	t.Setenv("QUERY_AUTH_URL", identity.URL)
	t.Setenv("QUERY_REALM", "ddae")
	t.Setenv("QUERY_ROLE", "sysadmin")
	t.Setenv("QUERY_USERNAME", "synthetic")
	t.Setenv("QUERY_PASSWORD", "synthetic")
	t.Setenv("ALLOW_INSECURE_TLS", "true")
	t.Setenv("QUERY_TLS_INSECURE_SKIP_VERIFY", "true")
	t.Setenv("DDAE_RESOURCE_MONITORING_ENABLED", "false")
	t.Setenv("DDAE_ALERT_MONITORING_ENABLED", "false")
	t.Setenv("DDAE_SOURCE_INSTANCE", "test")
	t.Setenv("STATE_DIR", t.TempDir())
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	client, err := New(cfg.Query, true)
	if err != nil {
		t.Fatal(err)
	}
	defer client.CloseIdleConnections()
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := client.Scope(context.Background()); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if logins.Load() != 1 {
		t.Fatalf("expected coalesced login, got %d", logins.Load())
	}
	client.http.Jar, _ = cookiejar.New(nil)
	if err := client.Scope(context.Background()); err != nil {
		t.Fatal(err)
	}
	if logins.Load() != 2 {
		t.Fatal("session did not renew")
	}
	if _, err := client.Get(context.Background(), "/ui/api/query/kill"); err == nil {
		t.Fatal("mutation path allowed")
	}
	cfg.Query.Insecure = false
	secure, err := New(cfg.Query, false)
	if err != nil {
		t.Fatal(err)
	}
	defer secure.CloseIdleConnections()
	if err := secure.Scope(context.Background()); err == nil {
		t.Fatal("untrusted TLS accepted")
	}
	engineCA := filepath.Join(t.TempDir(), "engine-ca.pem")
	authCA := filepath.Join(t.TempDir(), "auth-ca.pem")
	if err := os.WriteFile(engineCA, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: engine.Certificate().Raw}), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(authCA, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: identity.Certificate().Raw}), 0600); err != nil {
		t.Fatal(err)
	}
	cfg.Query.CAFile = engineCA
	cfg.Query.AuthCAFile = authCA
	trusted, err := New(cfg.Query, false)
	if err != nil {
		t.Fatal(err)
	}
	defer trusted.CloseIdleConnections()
	if err := trusted.Scope(context.Background()); err != nil {
		t.Fatal("custom CAs rejected", err)
	}
	cfg.Query.Insecure = true
	if _, err := New(cfg.Query, false); err == nil {
		t.Fatal("unguarded insecure accepted")
	}
	cfg.Query.Insecure = false
	cfg.Query.CAFile = "missing-ca.pem"
	if _, err := New(cfg.Query, false); err == nil {
		t.Fatal("missing CA accepted")
	}

}

func TestQueryRequestFailuresAreBounded(t *testing.T) {
	for _, tc := range []struct {
		status int
		body   string
		limit  int64
		want   int
	}{{403, "denied", 100, 1}, {500, "unavailable", 100, 2}, {200, strings.Repeat("x", 200), 100, 2}, {200, `{"allQueries":false}`, 100, 1}, {200, `null`, 100, 1}} {
		t.Run(fmt.Sprint(tc.status, tc.limit, len(tc.body)), func(t *testing.T) {
			calls := 0
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			u, _ := url.Parse(server.URL)
			c := &Client{base: u, auth: u, realm: "ddae", http: server.Client(), generation: 1, cfg: config.QueryConfig{Role: "sysadmin", ResponseMaxBytes: tc.limit, RetryMax: 1}}
			if err := c.Scope(context.Background()); err == nil {
				t.Fatal("failure accepted")
			}
			if calls != tc.want {
				t.Fatalf("calls=%d want=%d", calls, tc.want)
			}
		})
	}
}
func TestQueryLoginRejectsUnexpectedFormsAndRedirects(t *testing.T) {
	for _, body := range []string{
		`<form id="kc-form-login" method="post" action="https://evil.invalid/auth/realms/ddae/login-actions/authenticate"><input name="username"><input name="password"></form>`,
		`<form id="kc-form-login" method="post" action="/auth/realms/ddae/login-actions/authenticate"><input name="username"><input name="password"><input name="otp"></form>`,
		`<form id="kc-form-login" method="get" action="/auth/realms/ddae/login-actions/authenticate"><input name="username"><input name="password"></form>`,
		`<form id="kc-form-login" id="other" method="post"></form>`,
		`<html>SSO changed</html>`,
	} {
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "GET" {
				t.Error("credentials submitted to unsupported form")
			}
			fmt.Fprint(w, body)
		}))
		u, _ := url.Parse(server.URL)
		c := &Client{base: u, auth: u, realm: "ddae", http: server.Client(), cfg: config.QueryConfig{ResponseMaxBytes: 4096}}
		if err := c.authenticate(context.Background(), 0); err == nil {
			t.Error("unexpected login accepted")
		}
		server.Close()
	}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/ui/insights/login", 303) }))
	defer server.Close()
	u, _ := url.Parse(server.URL)
	hc := server.Client()
	hc.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	c := &Client{base: u, auth: u, realm: "ddae", http: hc, cfg: config.QueryConfig{ResponseMaxBytes: 4096}}
	target := *u
	target.Path = "/ui/insights/login"
	n := 0
	if _, _, err := c.loginChain(context.Background(), &target, "GET", nil, &n); err == nil || n != 8 {
		t.Fatal("redirect limit not enforced", n, err)
	}
}
