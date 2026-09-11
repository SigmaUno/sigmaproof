package server

import (
	"net/http/httptest"
	"testing"
)

func TestSkeletonNotReady(t *testing.T) {
	for path, want := range map[string]int{"/healthz": 200, "/readyz": 503, "/v1/ingest": 404, "/": 404} {
		rec := httptest.NewRecorder()
		Handler().ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		if rec.Code != want {
			t.Errorf("%s: got %d, want %d", path, rec.Code, want)
		}
	}
}
