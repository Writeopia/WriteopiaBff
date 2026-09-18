// Package bff implements the Writeopia Backend-for-Frontend: a stateless
// reverse proxy that bridges the webapp's HttpOnly session cookie into an
// Authorization header before forwarding requests to API Gateway.
//
// API Gateway can only read a JWT from a header or query param, never from a
// cookie, so this service exists purely to bridge that gap. It performs no
// JWT verification of its own — that happens at API Gateway (infrastructure
// level) and, for native/mobile clients, directly against the Authorization
// header they already send.
package bff

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
)

// AccessCookieName is the HttpOnly cookie set by the auth service for web
// clients (see writeopia_access in CookieAuthRouting.kt).
const AccessCookieName = "writeopia_access"

// NewProxy builds a reverse proxy that forwards every request to upstream
// unchanged, except that it injects an Authorization: Bearer header derived
// from the AccessCookieName cookie whenever the request has no Authorization
// header of its own.
func NewProxy(upstream *url.URL) *httputil.ReverseProxy {
	proxy := httputil.NewSingleHostReverseProxy(upstream)

	baseDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		baseDirector(req)

		// NewSingleHostReverseProxy rewrites the URL but leaves the Host
		// header as the original client Host (app.writeopia.io). Force it to
		// match upstream so API Gateway's own routing/host checks see the
		// host they expect.
		req.Host = upstream.Host

		if req.Header.Get("Authorization") == "" {
			if cookie, err := req.Cookie(AccessCookieName); err == nil && cookie.Value != "" {
				req.Header.Set("Authorization", "Bearer "+cookie.Value)
			}
		}
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("bff: upstream error for %s %s: %v", r.Method, r.URL.Path, err)
		w.WriteHeader(http.StatusBadGateway)
	}

	return proxy
}
