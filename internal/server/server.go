// Package server provides the development daemon's operational endpoints.
package server

import "net/http"

// Handler exposes liveness separately from readiness. The skeleton cannot ingest.
func Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("{\"status\":\"alive\"}\n"))
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("{\"status\":\"unavailable\",\"reason\":\"evidence engine not implemented\"}\n"))
	})
	return mux
}
