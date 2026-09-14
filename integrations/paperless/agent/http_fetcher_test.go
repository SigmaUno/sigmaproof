package agent

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPFetcherDownloadsRepresentations(t *testing.T) {
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.URL.String())
		if r.Header.Get("Authorization") != "Token secret-token" {
			t.Fatalf("authorization header not set")
		}
		if got := r.Header.Get("Accept"); !strings.Contains(got, "version=10") {
			t.Fatalf("missing API version in Accept: %q", got)
		}
		if r.URL.Path != "/paperless/api/documents/42/download" {
			t.Fatalf("wrong path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("version") != "v1" {
			t.Fatalf("missing version query: %s", r.URL.RawQuery)
		}
		if len(requests) == 1 && r.URL.Query().Get("original") != "true" {
			t.Fatalf("original representation not requested: %s", r.URL.RawQuery)
		}
		if len(requests) == 2 && r.URL.Query().Has("original") {
			t.Fatalf("archive representation should use Paperless default archive download: %s", r.URL.RawQuery)
		}
		w.Write([]byte("document bytes"))
	}))
	defer server.Close()
	fetcher := HTTPFetcher{BaseURL: server.URL + "/paperless", Token: "secret-token", Client: server.Client()}
	for _, rep := range []Representation{OriginalUpload, ArchiveOutput} {
		result, err := fetcher.Fetch(FetchRequest{Instance: "paperless-main", DocumentID: "42", Version: "v1", Representation: rep})
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(result.Document)
		result.Document.(io.Closer).Close()
		if err != nil || string(data) != "document bytes" || result.Version != "v1" {
			t.Fatalf("bad fetch: %q %+v %v", data, result, err)
		}
	}
}

func TestHTTPFetcherStatusAndConfigErrors(t *testing.T) {
	for _, tc := range []struct {
		status int
		want   error
	}{
		{http.StatusUnauthorized, ErrFetchUnauthorized},
		{http.StatusForbidden, ErrFetchUnauthorized},
		{http.StatusNotFound, ErrFetchNotFound},
		{http.StatusBadGateway, ErrFetchFailed},
	} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(tc.status)
		}))
		fetcher := HTTPFetcher{BaseURL: server.URL, Token: "secret-token", Client: server.Client()}
		_, err := fetcher.Fetch(FetchRequest{Instance: "paperless-main", DocumentID: "42", Version: "v1", Representation: OriginalUpload})
		server.Close()
		if !errors.Is(err, tc.want) {
			t.Fatalf("status %d: %v", tc.status, err)
		}
		if strings.Contains(err.Error(), "secret-token") {
			t.Fatal("error leaked token")
		}
	}
	bad := HTTPFetcher{BaseURL: "://bad", Token: "secret-token"}
	if _, err := bad.Fetch(FetchRequest{Instance: "paperless-main", DocumentID: "42", Version: "v1", Representation: OriginalUpload}); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("bad base URL accepted: %v", err)
	}
	bad = HTTPFetcher{BaseURL: "https://paperless.example", Token: ""}
	if _, err := bad.Fetch(FetchRequest{Instance: "paperless-main", DocumentID: "42", Version: "v1", Representation: OriginalUpload}); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("empty token accepted: %v", err)
	}
	bad = HTTPFetcher{BaseURL: "https://paperless.example", Token: "secret-token"}
	if _, err := bad.Fetch(FetchRequest{Instance: "paperless-main", DocumentID: "42", Version: "v1", Representation: "thumbnail"}); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("bad representation accepted: %v", err)
	}
}

func TestHTTPFetcherCustomAPIVersion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Accept"); !strings.Contains(got, "version=9") {
			t.Fatalf("missing custom API version: %q", got)
		}
		w.Write([]byte("document bytes"))
	}))
	defer server.Close()
	fetcher := HTTPFetcher{BaseURL: server.URL, Token: "secret-token", APIVersion: "9", Client: server.Client()}
	result, err := fetcher.Fetch(FetchRequest{Instance: "paperless-main", DocumentID: "42", Version: "v1", Representation: ArchiveOutput})
	if err != nil {
		t.Fatal(err)
	}
	result.Document.(io.Closer).Close()
}
