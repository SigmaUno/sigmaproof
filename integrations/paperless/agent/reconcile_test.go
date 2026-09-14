package agent

import (
	"errors"
	"testing"
)

type listSource struct {
	events []Event
	limit  int
	err    error
}

func (s *listSource) ListReady(limit int) ([]Event, error) {
	s.limit = limit
	if s.err != nil {
		return nil, s.err
	}
	return append([]Event(nil), s.events...), nil
}

func TestReconcilerReplaysMissedAndDuplicateEvents(t *testing.T) {
	store := openAgentStore(t)
	fetcher := &memoryFetcher{version: "v1", doc: []byte("document")}
	event := Event{Tenant: "tenant-a", Instance: "paperless-main", DocumentID: "42", Version: "v1", EventID: "event-1", Representation: OriginalUpload}
	source := &listSource{events: []Event{event, event, {ChangedFields: []string{"sigmaproof_status"}}}}
	reconciler := Reconciler{
		Source: source,
		Runner: Runner{Fetcher: fetcher, Ingester: store, EvidenceFields: []string{"sigmaproof_status"}, MaxBytes: 1024},
	}
	report, err := reconciler.Run()
	if err != nil {
		t.Fatal(err)
	}
	if report.Processed != 3 || report.Created != 1 || report.Existing != 1 || report.Skipped != 1 || report.Failed != 0 {
		t.Fatalf("wrong report: %+v", report)
	}
	if report.Items[0].EvidenceID == "" || report.Items[0].EvidenceID != report.Items[1].EvidenceID || !report.Items[0].Created || report.Items[1].Created || !report.Items[2].Skipped {
		t.Fatalf("wrong item results: %+v", report.Items)
	}
	if source.limit != DefaultReconcileLimit {
		t.Fatalf("default limit=%d", source.limit)
	}
}

func TestReconcilerPreservesItemFailures(t *testing.T) {
	store := openAgentStore(t)
	source := &listSource{events: []Event{
		{Tenant: "tenant-a", Instance: "paperless-main", DocumentID: "42", Version: "v1", EventID: "event-1", Representation: OriginalUpload},
		{Tenant: "", Instance: "paperless-main", DocumentID: "99", Version: "v1", EventID: "event-2", Representation: OriginalUpload},
	}}
	reconciler := Reconciler{
		Source: source,
		Runner: Runner{Fetcher: &memoryFetcher{version: "v1", doc: []byte("document")}, Ingester: store, MaxBytes: 1024},
		Limit:  10,
	}
	report, err := reconciler.Run()
	if err != nil {
		t.Fatal(err)
	}
	if source.limit != 10 || report.Processed != 2 || report.Created != 1 || report.Failed != 1 {
		t.Fatalf("wrong report: %+v limit=%d", report, source.limit)
	}
	if report.Items[1].Error == "" || report.Items[1].EvidenceID != "" {
		t.Fatalf("failure not preserved: %+v", report.Items[1])
	}
}

func TestReconcilerConfigurationAndSourceErrors(t *testing.T) {
	if _, err := (Reconciler{}).Run(); !errors.Is(err, ErrReconcilerNotConfigured) {
		t.Fatalf("missing source accepted: %v", err)
	}
	source := &listSource{}
	if _, err := (Reconciler{Source: source, Limit: -1}).Run(); !errors.Is(err, ErrReconcilerNotConfigured) {
		t.Fatalf("negative limit accepted: %v", err)
	}
	synthetic := errors.New("source unavailable")
	source = &listSource{err: synthetic}
	if _, err := (Reconciler{Source: source}).Run(); !errors.Is(err, synthetic) {
		t.Fatalf("source error hidden: %v", err)
	}
}
