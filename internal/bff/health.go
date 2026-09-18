package bff

import "net/http"

// HealthHandler responds to Cloud Run startup/liveness probes. It reports the
// process as healthy without touching the upstream, since the BFF holds no
// state and readiness of API Gateway is not this service's concern.
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}
