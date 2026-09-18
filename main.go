// Command bff runs the Writeopia BFF: a small stateless reverse proxy that
// bridges the webapp's HttpOnly session cookie into an Authorization header
// before forwarding requests to API Gateway. See internal/bff for the
// bridging logic.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/writeopia/writeopia-bff/internal/bff"
)

func main() {
	upstreamRaw := os.Getenv("UPSTREAM_BASE_URL")
	if upstreamRaw == "" {
		log.Fatal("UPSTREAM_BASE_URL environment variable is required")
	}

	upstream, err := url.Parse(upstreamRaw)
	if err != nil || upstream.Scheme == "" || upstream.Host == "" {
		log.Fatalf("UPSTREAM_BASE_URL %q is not a valid absolute URL: %v", upstreamRaw, err)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	serviceName := os.Getenv("SERVICE_NAME")
	if serviceName == "" {
		serviceName = "bff"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", bff.HealthHandler)
	mux.Handle("/", bff.NewProxy(upstream))

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           bff.LoggingMiddleware(mux),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		log.Printf("%s: listening on :%s, forwarding to %s", serviceName, port, upstream.String())
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Println("shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
}
