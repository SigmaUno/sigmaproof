package agent

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func contentVersion(data string) string {
	digest := sha256.Sum256([]byte(data))
	return hex.EncodeToString(digest[:])
}

func archiveMetadata(data string) string {
	checksum := md5.Sum([]byte(data))
	return fmt.Sprintf(`{"has_archive_version":true,"archive_checksum":"%x","original_metadata":[]}`, checksum)
}

func TestHTTPFetcherDownloadsRepresentations(t *testing.T) {
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.URL.String())
		if r.Header.Get("Authorization") != "Token secret-token" || r.Header.Get("Accept") != "application/json; version=5" {
			t.Error("incorrect auth or API version header")
		}
		if r.URL.Query().Has("version") {
			t.Error("content digest must never be sent as a Paperless version query")
		}
		switch r.URL.Path {
		case "/paperless/api/documents/42/metadata/":
			io.WriteString(w, archiveMetadata("archive bytes"))
		case "/paperless/api/documents/42/download/":
			if r.URL.Query().Get("original") == "true" {
				io.WriteString(w, "original bytes")
			} else {
				io.WriteString(w, "archive bytes")
			}
		default:
			t.Errorf("wrong path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	fetcher := HTTPFetcher{BaseURL: server.URL + "/paperless", Token: "secret-token", Client: server.Client(), MaxBytes: 14}
	for _, tc := range []struct {
		rep  Representation
		data string
	}{{OriginalUpload, "original bytes"}, {ArchiveOutput, "archive bytes"}} {
		version := contentVersion(tc.data)
		result, err := fetcher.Fetch(FetchRequest{Instance: "paperless-main", DocumentID: "42", Version: version, Representation: tc.rep})
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(result.Document)
		if err != nil || string(data) != tc.data || result.Version != version {
			t.Fatalf("bad fetch: %q %+v %v", data, result, err)
		}
	}
	want := []string{"/paperless/api/documents/42/download/?original=true", "/paperless/api/documents/42/metadata/", "/paperless/api/documents/42/download/", "/paperless/api/documents/42/metadata/"}
	if !reflect.DeepEqual(requests, want) {
		t.Fatalf("requests: %v", requests)
	}
}

func TestHTTPFetcherStatusAndConfigErrors(t *testing.T) {
	for _, tc := range []struct {
		status int
		want   error
	}{{401, ErrFetchUnauthorized}, {403, ErrFetchUnauthorized}, {404, ErrFetchNotFound}, {502, ErrFetchFailed}} {
		for _, rep := range []Representation{OriginalUpload, ArchiveOutput} {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(tc.status) }))
			fetcher := HTTPFetcher{BaseURL: server.URL, Token: "secret-token", Client: server.Client()}
			_, err := fetcher.Fetch(FetchRequest{Instance: "paperless-main", DocumentID: "42", Version: contentVersion("bytes"), Representation: rep})
			server.Close()
			if !errors.Is(err, tc.want) || strings.Contains(err.Error(), "secret-token") {
				t.Fatalf("status %d: %v", tc.status, err)
			}
		}
	}
	for _, change := range []func(*HTTPFetcher, *FetchRequest){
		func(f *HTTPFetcher, _ *FetchRequest) { f.BaseURL = "://bad" },
		func(f *HTTPFetcher, _ *FetchRequest) { f.BaseURL += "?original=true" },
		func(f *HTTPFetcher, _ *FetchRequest) { f.Token = "" },
		func(f *HTTPFetcher, _ *FetchRequest) { f.APIVersion = "10" },
		func(f *HTTPFetcher, _ *FetchRequest) { f.MaxBytes = -1 },
		func(f *HTTPFetcher, _ *FetchRequest) { f.MaxBytes = math.MaxInt64 },
		func(_ *HTTPFetcher, r *FetchRequest) { r.Version = "v1" },
		func(_ *HTTPFetcher, r *FetchRequest) { r.Version = strings.Repeat("g", 64) },
		func(_ *HTTPFetcher, r *FetchRequest) { r.Version = strings.ToUpper(r.Version) },
		func(_ *HTTPFetcher, r *FetchRequest) { r.Representation = "thumbnail" },
		func(_ *HTTPFetcher, r *FetchRequest) { r.DocumentID = "../42" },
	} {
		fetcher := HTTPFetcher{BaseURL: "https://paperless.example", Token: "secret-token"}
		req := FetchRequest{Instance: "paperless-main", DocumentID: "42", Version: contentVersion("bytes"), Representation: OriginalUpload}
		change(&fetcher, &req)
		if _, err := fetcher.Fetch(req); !errors.Is(err, ErrInvalidConfig) {
			t.Fatalf("bad configuration accepted: %+v %+v: %v", fetcher, req, err)
		}
	}
}

func TestHTTPFetcherRejectsArchiveFallbackAndMutation(t *testing.T) {
	for _, tc := range []struct {
		name     string
		before   string
		download string
		after    string
		want     error
		requests int
	}{
		{"absent", `{"has_archive_version":false,"archive_checksum":null}`, "original", "", ErrArchiveMissing, 1},
		{"missing flag", `{}`, "original", "", ErrFetchFailed, 1},
		{"missing checksum", `{"has_archive_version":true}`, "original", "", ErrFetchFailed, 1},
		{"invalid JSON", `{`, "original", "", ErrFetchFailed, 1},
		{"fallback with matching SHA256", archiveMetadata("archive"), "original", "", ErrVersionChanged, 2},
		{"removed during download", archiveMetadata("archive"), "archive", `{"has_archive_version":false}`, ErrArchiveMissing, 3},
		{"changed during download", archiveMetadata("archive"), "archive", archiveMetadata("new archive"), ErrVersionChanged, 3},
		{"metadata oversized", strings.Repeat(" ", (1<<20)+1), "archive", "", ErrOversizedFetch, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if strings.HasSuffix(r.URL.Path, "metadata/") {
					if calls == 1 {
						io.WriteString(w, tc.before)
					} else {
						io.WriteString(w, tc.after)
					}
				} else {
					io.WriteString(w, tc.download)
				}
			}))
			defer server.Close()
			fetcher := HTTPFetcher{BaseURL: server.URL, Token: "token", Client: server.Client()}
			result, err := fetcher.Fetch(FetchRequest{Instance: "main", DocumentID: "42", Version: contentVersion(tc.download), Representation: ArchiveOutput})
			if !errors.Is(err, tc.want) || result.Document != nil || calls != tc.requests {
				t.Fatalf("result=%+v err=%v calls=%d", result, err, calls)
			}
		})
	}
}

func TestHTTPFetcherRejectsMutableSourceBeforeIngestion(t *testing.T) {
	current := "first"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { io.WriteString(w, current) }))
	defer server.Close()
	store := openAgentStore(t)
	runner := Runner{Fetcher: HTTPFetcher{BaseURL: server.URL, Token: "token", Client: server.Client()}, Ingester: store}
	event := Event{Tenant: "tenant", Instance: "main", DocumentID: "42", Version: contentVersion(current), EventID: "1", Representation: OriginalUpload}
	if result, err := runner.Handle(event); err != nil || !result.Created {
		t.Fatalf("first capture: %+v %v", result, err)
	}
	current = "changed"
	for _, eventID := range []string{"1", "2"} {
		event.EventID = eventID
		if _, err := runner.Handle(event); !errors.Is(err, ErrVersionChanged) {
			t.Fatalf("mutable bytes accepted under old identity for event %s: %v", eventID, err)
		}
	}
	event.Version = contentVersion(current)
	if result, err := runner.Handle(event); err != nil || !result.Created {
		t.Fatalf("new content identity: %+v %v", result, err)
	}
}

type trackedBody struct {
	io.Reader
	closed bool
	read   int
}

func (b *trackedBody) Read(p []byte) (int, error) {
	n, err := b.Reader.Read(p)
	b.read += n
	return n, err
}
func (b *trackedBody) Close() error { b.closed = true; return nil }

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestHTTPFetcherClosesAndBoundsBodies(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		reader io.Reader
		want   error
	}{
		{"success", 200, strings.NewReader("1234"), nil},
		{"mismatch", 200, strings.NewReader("abcd"), ErrVersionChanged},
		{"overflow", 200, strings.NewReader(strings.Repeat("x", 100)), ErrOversizedFetch},
		{"read error", 200, errReader{}, nil},
		{"status error", 404, strings.NewReader("ignored"), ErrFetchNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := &trackedBody{Reader: tc.reader}
			client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: tc.status, Body: body, Header: make(http.Header)}, nil
			})}
			fetcher := HTTPFetcher{BaseURL: "http://paperless.example", Token: "token", Client: client, MaxBytes: 4}
			_, err := fetcher.Fetch(FetchRequest{Instance: "main", DocumentID: "42", Version: contentVersion("1234"), Representation: OriginalUpload})
			if !body.closed || body.read > 5 {
				t.Fatalf("closed=%v read=%d err=%v", body.closed, body.read, err)
			}
			if tc.name == "read error" {
				if err == nil {
					t.Fatal("read error ignored")
				}
			} else if !errors.Is(err, tc.want) {
				t.Fatalf("err=%v want=%v", err, tc.want)
			}
		})
	}
}
