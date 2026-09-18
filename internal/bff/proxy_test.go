package bff

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

// newTestProxy spins up a fake upstream running upstreamHandler and a BFF
// frontend in front of it, returning the frontend server for tests to hit.
func newTestProxy(t *testing.T, upstreamHandler http.HandlerFunc) *httptest.Server {
	t.Helper()
	upstream := httptest.NewServer(upstreamHandler)
	t.Cleanup(upstream.Close)

	target, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatalf("parse upstream url: %v", err)
	}

	frontend := httptest.NewServer(NewProxy(target))
	t.Cleanup(frontend.Close)

	return frontend
}

func TestCookieIsBridgedToAuthorizationHeader(t *testing.T) {
	var gotAuth, gotHost, gotPath, gotQuery string
	srv := newTestProxy(t, func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotHost = r.Host
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
	})

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/workspace/user?x=1", nil)
	req.AddCookie(&http.Cookie{Name: AccessCookieName, Value: "my-jwt"})
	req.Header.Set("Host", "app.writeopia.io")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if gotAuth != "Bearer my-jwt" {
		t.Fatalf("expected Authorization header to be bridged from cookie, got %q", gotAuth)
	}
	if gotPath != "/api/workspace/user" {
		t.Fatalf("expected path to be forwarded unchanged, got %q", gotPath)
	}
	if gotQuery != "x=1" {
		t.Fatalf("expected query to be forwarded unchanged, got %q", gotQuery)
	}
	_ = gotHost
}

func TestExistingAuthorizationHeaderIsNotOverwritten(t *testing.T) {
	var gotAuth string
	srv := newTestProxy(t, func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	})

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/workspace/user", nil)
	req.Header.Set("Authorization", "Bearer native-token")
	req.AddCookie(&http.Cookie{Name: AccessCookieName, Value: "web-cookie-token"})

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if gotAuth != "Bearer native-token" {
		t.Fatalf("expected existing Authorization header to win, got %q", gotAuth)
	}
}

func TestNoCookieNoHeaderMeansNoAuthorization(t *testing.T) {
	var gotAuth string
	authWasSet := false
	srv := newTestProxy(t, func(w http.ResponseWriter, r *http.Request) {
		gotAuth, authWasSet = r.Header.Get("Authorization"), r.Header.Get("Authorization") != ""
		w.WriteHeader(http.StatusOK)
	})

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/auth/login", nil)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if authWasSet {
		t.Fatalf("expected no Authorization header, got %q", gotAuth)
	}
}

func TestResponseIsForwardedUnchanged(t *testing.T) {
	srv := newTestProxy(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Set-Cookie", "writeopia_access=new-token; Path=/api; HttpOnly")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})

	resp, err := http.Get(srv.URL + "/api/docs/workspace/document")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", resp.StatusCode)
	}
	if resp.Header.Get("Set-Cookie") == "" {
		t.Fatalf("expected Set-Cookie header to be forwarded")
	}
}
