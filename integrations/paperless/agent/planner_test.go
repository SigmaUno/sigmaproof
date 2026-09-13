package agent

import (
	"errors"
	"testing"

	"github.com/SigmaUno/sigmaproof/pkg/proof/commitment"
)

func TestPlanIngestStableIdentity(t *testing.T) {
	event := Event{
		Tenant:         "tenant-a",
		Instance:       "paperless-main",
		DocumentID:     "42",
		Version:        "checksum:abc",
		EventID:        "webhook-1",
		Representation: OriginalUpload,
		ChangedFields:  []string{"archive_file"},
	}
	first, err := PlanIngest(event, []string{"sigmaproof_status"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := PlanIngest(event, []string{"sigmaproof_status"})
	if err != nil {
		t.Fatal(err)
	}
	if !first.Ingest || first.IdempotencyKey == "" || first.IdempotencyKey != second.IdempotencyKey {
		t.Fatalf("duplicate event was not idempotent: %+v %+v", first, second)
	}
	if first.Tenant != event.Tenant || first.Source.Instance != event.Instance || first.Source.Object != "paperless-document:42" || first.Source.Version != "checksum:abc:original" {
		t.Fatalf("wrong source identity: %+v", first.Source)
	}
	if first.Representation != commitment.Original {
		t.Fatalf("wrong representation: %+v", first.Representation)
	}
}

func TestPlanIngestDistinguishesVersionsEventsAndRepresentations(t *testing.T) {
	base := Event{Tenant: "tenant-a", Instance: "paperless-main", DocumentID: "42", Version: "v1", EventID: "event-1", Representation: OriginalUpload}
	first, err := PlanIngest(base, nil)
	if err != nil {
		t.Fatal(err)
	}
	version := base
	version.Version = "v2"
	nextVersion, err := PlanIngest(version, nil)
	if err != nil {
		t.Fatal(err)
	}
	event := base
	event.EventID = "event-2"
	nextEvent, err := PlanIngest(event, nil)
	if err != nil {
		t.Fatal(err)
	}
	archive := base
	archive.Representation = ArchiveOutput
	derived, err := PlanIngest(archive, nil)
	if err != nil {
		t.Fatal(err)
	}
	if first.IdempotencyKey == nextVersion.IdempotencyKey || first.IdempotencyKey == nextEvent.IdempotencyKey || first.IdempotencyKey == derived.IdempotencyKey {
		t.Fatal("changed version, event or representation reused an idempotency key")
	}
	if derived.Representation != commitment.Derived || derived.Source.Version != "v1:archive" {
		t.Fatalf("archive representation not explicit: %+v", derived)
	}
}

func TestPlanIngestIgnoresEvidenceWritebackLoop(t *testing.T) {
	event := Event{ChangedFields: []string{" sigmaproof_status ", "sigmaproof_url"}}
	plan, err := PlanIngest(event, []string{"sigmaproof_status", "sigmaproof_url"})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Ingest || plan.Reason != "evidence_writeback_only" {
		t.Fatalf("writeback-only event was not ignored: %+v", plan)
	}
	event.ChangedFields = append(event.ChangedFields, "archive_file")
	if _, err := PlanIngest(event, []string{"sigmaproof_status", "sigmaproof_url"}); !errors.Is(err, ErrInvalidEvent) {
		t.Fatal("mixed source change bypassed validation")
	}
}

func TestPlanIngestRejectsAmbiguousEvents(t *testing.T) {
	valid := Event{Tenant: "tenant-a", Instance: "paperless-main", DocumentID: "42", Version: "v1", EventID: "event-1", Representation: OriginalUpload}
	for _, mutate := range []func(*Event){
		func(e *Event) { e.Tenant = "" },
		func(e *Event) { e.Instance = "" },
		func(e *Event) { e.DocumentID = "" },
		func(e *Event) { e.Version = "" },
		func(e *Event) { e.EventID = "" },
		func(e *Event) { e.Representation = "thumbnail" },
		func(e *Event) { e.DocumentID = string([]byte{'4', 0, '2'}) },
	} {
		event := valid
		mutate(&event)
		if _, err := PlanIngest(event, nil); !errors.Is(err, ErrInvalidEvent) {
			t.Fatalf("accepted ambiguous event: %+v err=%v", event, err)
		}
	}
}
