package queryclient

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"html"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/crispkid/dell-ddae-metrics-exporter/internal/config"
)

type Client struct {
	base, auth *url.URL
	realm      string
	cfg        config.QueryConfig
	http       *http.Client
	mu         sync.Mutex
	generation uint64
}
type targetTransport struct {
	engine, identity         *http.Transport
	engineHost, identityHost string
}

func (t *targetTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.URL.Scheme != "https" {
		return nil, errors.New("query origin rejected")
	}
	if r.URL.Host == t.identityHost && strings.HasPrefix(r.URL.Path, "/auth/") {
		return t.identity.RoundTrip(r)
	}
	if r.URL.Host == t.engineHost {
		return t.engine.RoundTrip(r)
	}
	return nil, errors.New("query origin rejected")
}
func queryTLS(ca string, insecure bool) (*tls.Config, error) {
	roots, err := x509.SystemCertPool()
	if err != nil || roots == nil {
		roots = x509.NewCertPool()
	}
	if ca != "" {
		b, err := os.ReadFile(ca)
		if err != nil || !roots.AppendCertsFromPEM(b) {
			return nil, errors.New("query CA invalid")
		}
	}
	return &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, InsecureSkipVerify: insecure}, nil // #nosec G402 -- explicit dual opt-in checked by configuration and New.
}
func New(q config.QueryConfig, allowInsecure bool) (*Client, error) {
	if q.BaseURL == nil || q.AuthURL == nil || q.BaseURL.Scheme != "https" || q.AuthURL.Scheme != "https" || q.Username.Empty() || q.Password.Empty() || q.ResponseMaxBytes < 1 || q.ResponseMaxBytes > 64<<20 {
		return nil, errors.New("query configuration invalid")
	}
	if q.Insecure && (!allowInsecure || q.CAFile != "" || q.AuthCAFile != "") {
		return nil, errors.New("query TLS opt-in invalid")
	}
	e, err := queryTLS(q.CAFile, q.Insecure)
	if err != nil {
		return nil, err
	}
	a, err := queryTLS(q.AuthCAFile, q.Insecure)
	if err != nil {
		return nil, err
	}
	transport := func(tc *tls.Config) *http.Transport {
		return &http.Transport{TLSClientConfig: tc, TLSHandshakeTimeout: q.RequestTimeout, ResponseHeaderTimeout: q.RequestTimeout, MaxResponseHeaderBytes: 1 << 20, MaxIdleConnsPerHost: 8, MaxConnsPerHost: 32, IdleConnTimeout: 30 * time.Second}
	}
	jar, _ := cookiejar.New(nil)
	c := &Client{base: q.BaseURL, auth: q.AuthURL, realm: q.Realm, cfg: q}
	c.http = &http.Client{Transport: &targetTransport{transport(e), transport(a), q.BaseURL.Host, q.AuthURL.Host}, Jar: jar, Timeout: q.RequestTimeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return c, nil
}
func (c *Client) CloseIdleConnections() {
	if t, ok := c.http.Transport.(*targetTransport); ok {
		t.engine.CloseIdleConnections()
		t.identity.CloseIdleConnections()
	}
}
func (c *Client) allowedLogin(u *url.URL) bool {
	if u.Scheme != "https" || u.User != nil || u.Fragment != "" || u.RawPath != "" {
		return false
	}
	if u.Host == c.base.Host && (u.Path == "/ui/insights/login" || u.Path == "/ui/insights/login/proceed" || u.Path == "/ui/insights/" || u.Path == "/ui/insights/index.html" || u.Path == "/oauth2/callback") {
		return true
	}
	prefix := "/auth/realms/" + c.realm
	return u.Host == c.auth.Host && (u.Path == prefix+"/protocol/openid-connect/auth" || u.Path == prefix+"/login-actions/authenticate")
}
func (c *Client) request(ctx context.Context, method string, u *url.URL, body []byte, login bool) ([]byte, *http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, u.String(), bytes.NewReader(body))
	if err != nil {
		return nil, nil, errors.New("query request invalid")
	}
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if !login {
		req.Header.Set("X-Trino-Role", "system=ROLE{"+c.cfg.Role+"}")
		req.Header.Set("Accept", "application/json")
	}
	response, err := c.http.Do(req)
	if err != nil {
		return nil, nil, errors.New("query transport failed")
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, c.cfg.ResponseMaxBytes+1))
	if err != nil || int64(len(data)) > c.cfg.ResponseMaxBytes {
		return nil, response, errors.New("query response exceeds bounds")
	}
	return data, response, nil
}

// Redirects are handled explicitly: credentials cannot be replayed on a 307/308.
func (c *Client) loginChain(ctx context.Context, u *url.URL, method string, body []byte, redirects *int) ([]byte, *url.URL, error) {
	for {
		if !c.allowedLogin(u) {
			return nil, nil, errors.New("query login redirect rejected")
		}
		data, response, err := c.request(ctx, method, u, body, true)
		if err != nil {
			return nil, nil, err
		}
		if response.StatusCode == http.StatusOK {
			return data, u, nil
		}
		if response.StatusCode != 301 && response.StatusCode != 302 && response.StatusCode != 303 && response.StatusCode != 307 && response.StatusCode != 308 {
			return nil, nil, errors.New("query login rejected")
		}
		if method == http.MethodPost && (response.StatusCode == 307 || response.StatusCode == 308) {
			return nil, nil, errors.New("query credential redirect rejected")
		}
		if *redirects >= 8 {
			return nil, nil, errors.New("query login redirect limit")
		}
		*redirects++
		next, err := response.Location()
		if err != nil {
			return nil, nil, errors.New("query login location invalid")
		}
		method = http.MethodGet
		body = nil
		u = next
	}
}

var formRE = regexp.MustCompile(`(?is)<form\b([^>]*)>(.*?)</form\s*>`)
var attrRE = regexp.MustCompile(`(?is)([a-zA-Z][a-zA-Z0-9_-]*)\s*=\s*(?:"([^"]*)"|'([^']*)')`)
var inputRE = regexp.MustCompile(`(?is)<input\b([^>]*)>`)

func attributes(s string) (map[string]string, error) {
	m := map[string]string{}
	for _, a := range attrRE.FindAllStringSubmatch(s, -1) {
		k := strings.ToLower(a[1])
		if _, ok := m[k]; ok {
			return nil, errors.New("query login form ambiguous")
		}
		v := a[2]
		if v == "" {
			v = a[3]
		}
		m[k] = html.UnescapeString(v)
	}
	return m, nil
}
func (c *Client) authenticate(ctx context.Context, previous uint64) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.generation != previous {
		return nil
	}
	u := *c.base
	u.Path = "/ui/insights/login"
	redirects := 0
	data, at, err := c.loginChain(ctx, &u, http.MethodGet, nil, &redirects)
	if err != nil {
		return err
	}
	if at.Host == c.base.Host && at.Path == "/ui/insights/" {
		c.generation++
		return nil
	}
	var action *url.URL
	forms := formRE.FindAllSubmatch(data, -1)
	for _, f := range forms {
		attrs, err := attributes(string(f[1]))
		if err != nil {
			return err
		}
		if attrs["id"] != "kc-form-login" {
			continue
		}
		if action != nil || strings.ToLower(attrs["method"]) != "post" {
			return errors.New("query login form unsupported")
		}
		names := map[string]bool{}
		for _, i := range inputRE.FindAllSubmatch(f[2], -1) {
			a, err := attributes(string(i[1]))
			if err != nil {
				return err
			}
			name := a["name"]
			if name != "username" && name != "password" && name != "login" {
				return errors.New("query login additional input unsupported")
			}
			if names[name] {
				return errors.New("query login duplicate input")
			}
			names[name] = true
		}
		if !names["username"] || !names["password"] {
			return errors.New("query login fields missing")
		}
		action, err = at.Parse(attrs["action"])
		if err != nil {
			return errors.New("query login action invalid")
		}
	}
	if action == nil || !c.allowedLogin(action) || action.Host != c.auth.Host || action.Path != "/auth/realms/"+c.realm+"/login-actions/authenticate" {
		return errors.New("query password destination rejected")
	}
	form := url.Values{"username": {c.cfg.Username.Value()}, "password": {c.cfg.Password.Value()}, "login": {"Sign In"}}
	_, at, err = c.loginChain(ctx, action, http.MethodPost, []byte(form.Encode()), &redirects)
	if err != nil {
		return err
	}
	if at.Host != c.base.Host || at.Path != "/ui/insights/" {
		return errors.New("query authentication incomplete")
	}
	c.generation++
	return nil
}
func (c *Client) Get(ctx context.Context, path string) ([]byte, error) {
	prefix := "/ui/api/insights/"
	allowed := path == prefix+"cluster/info" || path == prefix+"overview/queries" || path == prefix+"history/queries"
	if strings.HasPrefix(path, prefix+"history/queries/") {
		allowed = ValidID(strings.TrimPrefix(path, prefix+"history/queries/"))
	}
	if !allowed {
		return nil, errors.New("query operation not allowed")
	}
	return c.get(ctx, path, nil)
}
func (c *Client) get(ctx context.Context, path string, params url.Values) ([]byte, error) {
	reauthenticated := false
	for attempt := 0; attempt <= c.cfg.RetryMax; attempt++ {
		c.mu.Lock()
		generation := c.generation
		c.mu.Unlock()
		u := *c.base
		u.Path = path
		u.RawQuery = params.Encode()
		data, response, err := c.request(ctx, http.MethodGet, &u, nil, false)
		if err == nil && response.StatusCode == 200 {
			return data, nil
		}
		if err == nil && response.StatusCode == 401 && !reauthenticated {
			if err = c.authenticate(ctx, generation); err != nil {
				return nil, err
			}
			reauthenticated = true
			attempt--
			continue
		}
		if err == nil && response.StatusCode != 429 && response.StatusCode < 500 {
			return nil, errors.New("query access rejected")
		}
		if attempt == c.cfg.RetryMax {
			return nil, errors.New("query request failed")
		}
		timer := time.NewTimer(time.Duration(100*(1<<attempt)) * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	return nil, errors.New("query request failed")
}
func (c *Client) Scope(ctx context.Context) error {
	b, err := c.Get(ctx, "/ui/api/insights/cluster/info")
	if err != nil {
		return err
	}
	var s struct {
		All *bool `json:"allQueries"`
	}
	if decode(b, &s) != nil || s.All == nil || !*s.All {
		return errors.New("query all-query scope required")
	}
	return nil
}

// HistoryWindow encodes a fixed Insights history operation, never an arbitrary URL.
func (c *Client) HistoryWindow(ctx context.Context, start, end time.Time) ([]byte, error) {
	if start.IsZero() || !end.After(start) {
		return nil, errors.New("invalid history time bounds")
	}
	start = start.UTC().Truncate(time.Second)
	end = end.UTC().Add(time.Second - time.Nanosecond).Truncate(time.Second)
	filter, _ := json.Marshal(struct {
		Start string `json:"startDate"`
		End   string `json:"endDate"`
	}{start.Format(time.RFC3339), end.Format(time.RFC3339)})
	return c.get(ctx, "/ui/api/insights/history/queries", url.Values{"filter": {string(filter)}, "sortBy": {"createDate"}, "sortOrder": {"desc"}})
}
