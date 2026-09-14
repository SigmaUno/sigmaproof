package agent

import (
	"bytes"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/SigmaUno/sigmaproof/internal/storage"
)

type memoryFetcher struct {
	mu      sync.Mutex
	version string
	doc     []byte
	err     error
	calls   int
}

func (f *memoryFetcher) Fetch(FetchRequest) (FetchResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.err != nil {
		return FetchResult{}, f.err
	}
	return FetchResult{Version: f.version, Document: bytes.NewReader(append([]byte(nil), f.doc...))}, nil
}

func TestRunnerIngestsIdempotently(t *testing.T) {
	store := openAgentStore(t)
	fetcher := &memoryFetcher{version: "v1", doc: []byte("archive bytes")}
	runner := Runner{Fetcher: fetcher, Ingester: store, MaxBytes: 1024}
	event := Event{Tenant: "tenant-a", Instance: "paperless-main", DocumentID: "42", Version: "v1", EventID: "event-1", Representation: ArchiveOutput}
	first, err := runner.Handle(event)
	if err != nil {
		t.Fatal(err)
	}
	second, err := runner.Handle(event)
	if err != nil {
		t.Fatal(err)
	}
	if !first.Created || second.Created || first.EvidenceID == "" || first.EvidenceID != second.EvidenceID {
		t.Fatalf("event was not idempotent: %+v %+v", first, second)
	}
	if first.Plan.Source.Object != "paperless-document:42" || first.Plan.Source.Version != "v1:archive" {
		t.Fatalf("wrong source identity: %+v", first.Plan.Source)
	}
	if fetcher.calls != 2 {
		t.Fatalf("fetch calls=%d", fetcher.calls)
	}
}

func TestRunnerRejectsChangedRetryBytes(t *testing.T) {
	store := openAgentStore(t)
	fetcher := &memoryFetcher{version: "v1", doc: []byte("first")}
	runner := Runner{Fetcher: fetcher, Ingester: store, MaxBytes: 1024}
	event := Event{Tenant: "tenant-a", Instance: "paperless-main", DocumentID: "42", Version: "v1", EventID: "event-1", Representation: OriginalUpload}
	if _, err := runner.Handle(event); err != nil {
		t.Fatal(err)
	}
	fetcher.doc = []byte("changed")
	if _, err := runner.Handle(event); !errors.Is(err, storage.ErrConflict) {
		t.Fatalf("changed retry did not conflict: %v", err)
	}
}

func TestRunnerSkipsWritebackOnlyEventsWithoutFetching(t *testing.T) {
	store := openAgentStore(t)
	fetcher := &memoryFetcher{version: "v1", doc: []byte("ignored")}
	runner := Runner{Fetcher: fetcher, Ingester: store, EvidenceFields: []string{"sigmaproof_status", "sigmaproof_url"}}
	result, err := runner.Handle(Event{ChangedFields: []string{"sigmaproof_status", "sigmaproof_url"}})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Skipped || result.Plan.Reason != "evidence_writeback_only" || fetcher.calls != 0 {
		t.Fatalf("write-back loop was not skipped: %+v calls=%d", result, fetcher.calls)
	}
}

func TestRunnerDetectsMutableFetchAndBounds(t *testing.T) {
	store := openAgentStore(t)
	runner := Runner{Fetcher: &memoryFetcher{version: "v2", doc: []byte("changed")}, Ingester: store, MaxBytes: 4}
	event := Event{Tenant: "tenant-a", Instance: "paperless-main", DocumentID: "42", Version: "v1", EventID: "event-1", Representation: OriginalUpload}
	if _, err := runner.Handle(event); !errors.Is(err, ErrVersionChanged) {
		t.Fatalf("version change not detected: %v", err)
	}
	runner.Fetcher = &memoryFetcher{version: "v1", doc: []byte("12345")}
	if _, err := runner.Handle(event); !errors.Is(err, ErrOversizedFetch) {
		t.Fatalf("oversized fetch not detected: %v", err)
	}
	runner.Fetcher = fetchFunc(func(FetchRequest) (FetchResult, error) {
		return FetchResult{Version: "v1"}, nil
	})
	if _, err := runner.Handle(event); !errors.Is(err, ErrMissingDocument) {
		t.Fatalf("missing document not detected: %v", err)
	}
}

func TestRunnerValidationAndFetchFailures(t *testing.T) {
	store := openAgentStore(t)
	synthetic := errors.New("paperless unavailable")
	runner := Runner{Fetcher: &memoryFetcher{version: "v1", err: synthetic}, Ingester: store}
	event := Event{Tenant: "tenant-a", Instance: "paperless-main", DocumentID: "42", Version: "v1", EventID: "event-1", Representation: OriginalUpload}
	if _, err := runner.Handle(event); !errors.Is(err, synthetic) {
		t.Fatalf("fetch error hidden: %v", err)
	}
	runner = Runner{Fetcher: fetchFunc(func(FetchRequest) (FetchResult, error) {
		return FetchResult{Version: "v1", Document: errReader{}}, nil
	}), Ingester: store}
	if _, err := runner.Handle(event); err == nil {
		t.Fatal("fetch read error ignored")
	}
	runner = Runner{Fetcher: &memoryFetcher{version: "v1", doc: []byte("doc")}}
	if _, err := runner.Handle(event); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("missing ingester accepted: %v", err)
	}
	event.Tenant = ""
	if _, err := runner.Handle(event); !errors.Is(err, ErrInvalidEvent) {
		t.Fatalf("invalid event accepted: %v", err)
	}
}

type fetchFunc func(FetchRequest) (FetchResult, error)

func (f fetchFunc) Fetch(req FetchRequest) (FetchResult, error) { return f(req) }

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }

func openAgentStore(t *testing.T) *storage.Store {
	t.Helper()
	store, err := storage.Open(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestBoundedBytesAcceptsExactLimit(t *testing.T) {
	data, err := boundedBytes(strings.NewReader("1234"), 4)
	if err != nil || string(data) != "1234" {
		t.Fatalf("exact limit rejected: %q %v", data, err)
	}
	if _, err := boundedBytes(io.MultiReader(strings.NewReader("12"), strings.NewReader("345")), 4); !errors.Is(err, ErrOversizedFetch) {
		t.Fatalf("stream overflow not detected: %v", err)
	}
}
