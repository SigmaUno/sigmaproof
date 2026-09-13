// Package server provides the development daemon's operational endpoints.
package server

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"github.com/SigmaUno/sigmaproof/internal/storage"
	"github.com/SigmaUno/sigmaproof/pkg/proof"
	"github.com/SigmaUno/sigmaproof/pkg/proof/commitment"
)

const (
	maxJSONBytes     = 96 << 20
	maxDocumentBytes = 64 << 20
)

type engine interface {
	IngestDocument(storage.DocumentIngestRequest) (storage.Evidence, bool, error)
	Evidence(string, string) (storage.Evidence, error)
	Batch(string, string) (storage.FrozenBatch, error)
	Freeze(string, string, int) (storage.FrozenBatch, error)
	Outbox(string, int) ([]storage.Submission, error)
	UnanchoredPackage(string, string) (proof.Package, error)
}

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

// HandlerWithStore exposes the experimental local evidence engine. The caller is
// responsible for authenticating tenants before this is reachable by users.
func HandlerWithStore(store *storage.Store) http.Handler {
	return handlerWithEngine(store)
}

func handlerWithEngine(e engine) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", requireMethod(http.MethodGet, writeJSON(http.StatusOK, map[string]string{"status": "alive"})))
	mux.HandleFunc("/readyz", requireMethod(http.MethodGet, writeJSON(http.StatusOK, map[string]string{"status": "ready"})))
	mux.HandleFunc("/v1/ingest", requireMethod(http.MethodPost, ingest(e)))
	mux.HandleFunc("/v1/batches/freeze", requireMethod(http.MethodPost, freeze(e)))
	mux.HandleFunc("/v1/batches/{id}", requireMethod(http.MethodGet, batchStatus(e)))
	mux.HandleFunc("/v1/outbox", requireMethod(http.MethodGet, outbox(e)))
	mux.HandleFunc("/v1/evidence/{id}", requireMethod(http.MethodGet, evidenceStatus(e)))
	mux.HandleFunc("/v1/evidence/{id}/package", requireMethod(http.MethodGet, packageExport(e)))
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		errorJSON(w, http.StatusNotFound, "not_found", "endpoint was not found")
	})
	return mux
}

func requireMethod(method string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != method {
			w.Header().Set("Allow", method)
			errorJSON(w, http.StatusMethodNotAllowed, "method_not_allowed", "method is not allowed for this endpoint")
			return
		}
		next(w, r)
	}
}

type sourceJSON struct {
	Instance string `json:"instance"`
	Object   string `json:"object"`
	Version  string `json:"version"`
}

type ingestRequest struct {
	Tenant         string     `json:"tenant"`
	IdempotencyKey string     `json:"idempotency_key"`
	Source         sourceJSON `json:"source"`
	Representation string     `json:"representation"`
	DocumentBase64 string     `json:"document_base64"`
}

func ingest(e engine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req ingestRequest
		if !readJSON(w, r, &req) {
			return
		}
		rep, ok := representation(req.Representation)
		if !ok {
			errorJSON(w, http.StatusBadRequest, "invalid_request", "representation must be original or derived")
			return
		}
		document, err := base64.StdEncoding.Strict().DecodeString(req.DocumentBase64)
		if err != nil || len(document) > maxDocumentBytes {
			errorJSON(w, http.StatusBadRequest, "invalid_request", "document_base64 is invalid or exceeds the byte limit")
			return
		}
		evidence, created, err := e.IngestDocument(storage.DocumentIngestRequest{
			Tenant:         req.Tenant,
			Key:            req.IdempotencyKey,
			Source:         storage.Source{Instance: req.Source.Instance, Object: req.Source.Object, Version: req.Source.Version},
			Representation: rep,
			Document:       bytes.NewReader(document),
			MaxBytes:       maxDocumentBytes,
		})
		if err != nil {
			writeStorageError(w, err)
			return
		}
		status := http.StatusOK
		if created {
			status = http.StatusCreated
		}
		writeJSON(status, map[string]any{
			"evidence_id": evidence.ID,
			"created":     created,
			"sequence":    evidence.Sequence,
			"batch_id":    emptyNil(evidence.BatchID),
		})(w, r)
	}
}

type freezeRequest struct {
	Tenant     string `json:"tenant"`
	RequestKey string `json:"request_key"`
	Limit      int    `json:"limit"`
}

func freeze(e engine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req freezeRequest
		if !readJSON(w, r, &req) {
			return
		}
		b, err := e.Freeze(req.Tenant, req.RequestKey, req.Limit)
		if err != nil {
			writeStorageError(w, err)
			return
		}
		writeJSON(http.StatusCreated, batchResponse(b))(w, r)
	}
}

func batchStatus(e engine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		b, err := e.Batch(r.URL.Query().Get("tenant"), r.PathValue("id"))
		if err != nil {
			writeStorageError(w, err)
			return
		}
		writeJSON(http.StatusOK, batchResponse(b))(w, r)
	}
}

func outbox(e engine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit, ok := queryLimit(w, r, storage.MaxBatchEntries)
		if !ok {
			return
		}
		items, err := e.Outbox(r.URL.Query().Get("tenant"), limit)
		if err != nil {
			writeStorageError(w, err)
			return
		}
		type item struct {
			BatchID      string `json:"batch_id"`
			ManifestHex  string `json:"manifest_hex"`
			State        string `json:"state"`
			AnchorStatus string `json:"anchor_status"`
			Reference    any    `json:"reference"`
		}
		resp := struct {
			Submissions []item `json:"submissions"`
		}{Submissions: []item{}}
		for _, s := range items {
			resp.Submissions = append(resp.Submissions, item{s.BatchID, hex.EncodeToString(s.Manifest), batchState(s.State), anchorStatus(s.State), emptyNil(s.Ref)})
		}
		writeJSON(http.StatusOK, resp)(w, r)
	}
}

func evidenceStatus(e engine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		record, err := e.Evidence(r.URL.Query().Get("tenant"), r.PathValue("id"))
		if err != nil {
			writeStorageError(w, err)
			return
		}
		state := "pending_batch"
		if record.BatchID != "" {
			state = "pending_submission"
		}
		writeJSON(http.StatusOK, map[string]any{
			"evidence_id": record.ID,
			"sequence":    record.Sequence,
			"state":       state,
			"batch_id":    emptyNil(record.BatchID),
			"batch_index": batchIndex(record),
			"source": sourceJSON{
				Instance: record.Source.Instance,
				Object:   record.Source.Object,
				Version:  record.Source.Version,
			},
			"representation": representationName(record.Witness.Representation),
		})(w, r)
	}
}

func packageExport(e engine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, err := e.UnanchoredPackage(r.URL.Query().Get("tenant"), r.PathValue("id"))
		if err != nil {
			writeStorageError(w, err)
			return
		}
		data, err := p.MarshalBinary()
		if err != nil {
			writeStorageError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/vnd.sigmaproof")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	}
}

func readJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		errorJSON(w, http.StatusUnsupportedMediaType, "unsupported_media_type", "content type must be application/json")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBytes)
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(target); err != nil {
		errorJSON(w, http.StatusBadRequest, "invalid_json", "request body must be bounded JSON with known fields")
		return false
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		errorJSON(w, http.StatusBadRequest, "invalid_json", "request body must contain one JSON value")
		return false
	}
	return true
}

func queryLimit(w http.ResponseWriter, r *http.Request, fallback int) (int, bool) {
	value := r.URL.Query().Get("limit")
	if value == "" {
		return fallback, true
	}
	limit, err := strconv.Atoi(value)
	if err != nil || limit < 1 || limit > storage.MaxBatchEntries || limit == math.MaxInt {
		errorJSON(w, http.StatusBadRequest, "invalid_request", "limit is outside the accepted range")
		return 0, false
	}
	return limit, true
}

func representation(value string) (commitment.Representation, bool) {
	switch strings.ToLower(value) {
	case "original":
		return commitment.Original, true
	case "derived":
		return commitment.Derived, true
	default:
		return 0, false
	}
}

func representationName(value commitment.Representation) string {
	switch value {
	case commitment.Original:
		return "original"
	case commitment.Derived:
		return "derived"
	default:
		return "unsupported"
	}
}

func writeStorageError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, storage.ErrInvalid):
		errorJSON(w, http.StatusBadRequest, "invalid_request", "request violates input limits or required fields")
	case errors.Is(err, storage.ErrConflict):
		errorJSON(w, http.StatusConflict, "conflict", "idempotency key, source version or batch request conflicts with existing evidence")
	case errors.Is(err, storage.ErrNotFound):
		errorJSON(w, http.StatusNotFound, "not_found", "record was not found for this tenant")
	case errors.Is(err, storage.ErrNotBatched):
		errorJSON(w, http.StatusConflict, "not_batched", "evidence has no frozen batch yet")
	case errors.Is(err, storage.ErrEmpty):
		errorJSON(w, http.StatusConflict, "no_pending_evidence", "tenant has no pending evidence to freeze")
	default:
		errorJSON(w, http.StatusInternalServerError, "internal_error", "evidence engine failed closed")
	}
}

func writeJSON(status int, value any) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(value)
	}
}

func errorJSON(w http.ResponseWriter, status int, code, message string) {
	writeJSON(status, map[string]string{"error": code, "message": message})(w, nil)
}

func emptyNil(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func batchIndex(record storage.Evidence) any {
	if record.BatchID == "" {
		return nil
	}
	return record.Index
}

func batchResponse(b storage.FrozenBatch) map[string]any {
	return map[string]any{
		"batch_id":      b.ID,
		"state":         batchState(b.SubmissionState),
		"evidence_ids":  b.EvidenceIDs,
		"member_count":  len(b.EvidenceIDs),
		"limit":         b.Limit,
		"manifest_hex":  hex.EncodeToString(b.Manifest),
		"anchor_status": anchorStatus(b.SubmissionState),
		"reference":     emptyNil(b.SubmissionRef),
	}
}

func batchState(submissionState string) string {
	if submissionState == storage.SubmissionSubmitted {
		return "submitted"
	}
	return "pending_submission"
}

func anchorStatus(submissionState string) string {
	switch submissionState {
	case storage.SubmissionUnknown:
		return "unknown"
	case storage.SubmissionSubmitted:
		return "submitted"
	default:
		return "not_submitted"
	}
}
