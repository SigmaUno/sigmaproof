// Package agent contains Paperless integration planning helpers.
package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/SigmaUno/sigmaproof/internal/storage"
	"github.com/SigmaUno/sigmaproof/pkg/proof/commitment"
)

type Representation string

const (
	OriginalUpload Representation = "original"
	ArchiveOutput  Representation = "archive"
)

type Event struct {
	Tenant         string
	Instance       string
	DocumentID     string
	Version        string
	EventID        string
	Representation Representation
	ChangedFields  []string
}

type Plan struct {
	Ingest         bool
	Reason         string
	Tenant         string
	IdempotencyKey string
	Source         storage.Source
	Representation commitment.Representation
}

var ErrInvalidEvent = errors.New("invalid paperless event")

// PlanIngest converts an authenticated Paperless event into one deterministic
// ingestion request description. It never fetches bytes or trusts mutable names.
func PlanIngest(event Event, evidenceFields []string) (Plan, error) {
	if writebackOnly(event.ChangedFields, evidenceFields) {
		return Plan{Ingest: false, Reason: "evidence_writeback_only"}, nil
	}
	if !validText(event.Tenant) || !validText(event.Instance) || !validText(event.DocumentID) || !validText(event.Version) || !validText(event.EventID) {
		return Plan{}, ErrInvalidEvent
	}
	rep, sourceRep, ok := representation(event.Representation)
	if !ok {
		return Plan{}, ErrInvalidEvent
	}
	source := storage.Source{
		Instance: event.Instance,
		Object:   "paperless-document:" + event.DocumentID,
		Version:  event.Version + ":" + string(event.Representation),
	}
	key := stableKey(event.Tenant, event.Instance, event.DocumentID, event.Version, string(event.Representation), event.EventID)
	return Plan{
		Ingest:         true,
		Reason:         "selected_representation_ready",
		Tenant:         event.Tenant,
		IdempotencyKey: key,
		Source:         source,
		Representation: rep,
	}, validateSource(source, sourceRep)
}

func representation(value Representation) (commitment.Representation, string, bool) {
	switch value {
	case OriginalUpload:
		return commitment.Original, "original", true
	case ArchiveOutput:
		return commitment.Derived, "archive", true
	default:
		return 0, "", false
	}
}

func stableKey(parts ...string) string {
	h := sha256.New()
	for _, part := range parts {
		h.Write([]byte{0})
		h.Write([]byte(part))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func writebackOnly(changed, evidenceFields []string) bool {
	if len(changed) == 0 || len(evidenceFields) == 0 {
		return false
	}
	allowed := map[string]bool{}
	for _, field := range evidenceFields {
		allowed[strings.TrimSpace(field)] = true
	}
	for _, field := range changed {
		if !allowed[strings.TrimSpace(field)] {
			return false
		}
	}
	return true
}

func validateSource(source storage.Source, rep string) error {
	if !validText(source.Object) || !validText(source.Version) || !strings.HasSuffix(source.Version, ":"+rep) {
		return ErrInvalidEvent
	}
	return nil
}

func validText(value string) bool {
	return len(value) > 0 && len(value) <= 256 && utf8.ValidString(value) && !strings.ContainsRune(value, 0)
}
