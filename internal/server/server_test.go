package server

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/SigmaUno/sigmaproof/internal/storage"
	"github.com/SigmaUno/sigmaproof/pkg/proof"
	"github.com/SigmaUno/sigmaproof/pkg/proof/batch"
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

func TestEngineHTTPIngestFreezeOutboxAndExport(t *testing.T) {
	s, err := storage.Open(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	handler := HandlerWithStore(s)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/missing", nil))
	if rec.Code != 404 || !strings.Contains(rec.Body.String(), "not_found") || rec.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("fallback code=%d content-type=%q body=%s", rec.Code, rec.Header().Get("Content-Type"), rec.Body.String())
	}
	doc := []byte("invoice bytes")
	ingestBody := map[string]any{
		"tenant":          "tenant-a",
		"idempotency_key": "paperless:1:v1",
		"source":          map[string]string{"instance": "paperless", "object": "1", "version": "v1"},
		"representation":  "original",
		"document_base64": base64.StdEncoding.EncodeToString(doc),
	}
	rec = postJSON(t, handler, "/v1/ingest", ingestBody)
	if rec.Code != 201 {
		t.Fatalf("ingest code=%d body=%s", rec.Code, rec.Body.String())
	}
	assertNoPrivateMaterial(t, rec.Body.String(), doc)
	var ingested struct {
		EvidenceID string  `json:"evidence_id"`
		Created    bool    `json:"created"`
		BatchID    *string `json:"batch_id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &ingested); err != nil {
		t.Fatal(err)
	}
	if ingested.EvidenceID == "" || !ingested.Created || ingested.BatchID != nil {
		t.Fatalf("bad ingest response: %+v", ingested)
	}
	rec = postJSON(t, handler, "/v1/ingest", ingestBody)
	if rec.Code != 200 {
		t.Fatalf("idempotent retry code=%d body=%s", rec.Code, rec.Body.String())
	}
	assertNoPrivateMaterial(t, rec.Body.String(), doc)
	var retried struct {
		EvidenceID string `json:"evidence_id"`
		Created    bool   `json:"created"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &retried); err != nil {
		t.Fatal(err)
	}
	if retried.EvidenceID != ingested.EvidenceID || retried.Created {
		t.Fatal("retry changed evidence")
	}
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/evidence/"+ingested.EvidenceID+"?tenant=tenant-a", nil))
	if rec.Code != 200 {
		t.Fatalf("pre-freeze status code=%d body=%s", rec.Code, rec.Body.String())
	}
	var pending struct {
		EvidenceID     string  `json:"evidence_id"`
		State          string  `json:"state"`
		BatchID        *string `json:"batch_id"`
		BatchIndex     *uint64 `json:"batch_index"`
		Representation string  `json:"representation"`
		Source         struct {
			Instance string `json:"instance"`
			Object   string `json:"object"`
			Version  string `json:"version"`
		} `json:"source"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &pending); err != nil {
		t.Fatal(err)
	}
	if pending.EvidenceID != ingested.EvidenceID || pending.State != "pending_batch" || pending.BatchID != nil || pending.BatchIndex != nil || pending.Representation != "original" || pending.Source.Object != "1" {
		t.Fatalf("bad pre-freeze status: %+v", pending)
	}
	assertNoPrivateMaterial(t, rec.Body.String(), doc)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/evidence/"+ingested.EvidenceID+"/package?tenant=tenant-a", nil))
	if rec.Code != 409 || !strings.Contains(rec.Body.String(), "not_batched") {
		t.Fatalf("unbatched export code=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = postJSON(t, handler, "/v1/batches/freeze", map[string]any{"tenant": "tenant-a", "request_key": "batch-1", "limit": 10})
	if rec.Code != 201 {
		t.Fatalf("freeze code=%d body=%s", rec.Code, rec.Body.String())
	}
	assertNoPrivateMaterial(t, rec.Body.String(), doc)
	var frozen struct {
		BatchID      string   `json:"batch_id"`
		EvidenceIDs  []string `json:"evidence_ids"`
		MemberCount  int      `json:"member_count"`
		Limit        int      `json:"limit"`
		ManifestHex  string   `json:"manifest_hex"`
		State        string   `json:"state"`
		AnchorStatus string   `json:"anchor_status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &frozen); err != nil {
		t.Fatal(err)
	}
	manifest, err := hex.DecodeString(frozen.ManifestHex)
	if err != nil {
		t.Fatal(err)
	}
	if frozen.BatchID == "" || frozen.State != "pending_submission" || frozen.AnchorStatus != "not_submitted" || frozen.MemberCount != 1 || frozen.Limit != 10 || len(frozen.EvidenceIDs) != 1 || frozen.EvidenceIDs[0] != ingested.EvidenceID {
		t.Fatalf("bad freeze response: %+v", frozen)
	}
	if _, err := batch.Parse(manifest); err != nil {
		t.Fatal(err)
	}
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/batches/"+frozen.BatchID+"?tenant=tenant-a", nil))
	if rec.Code != 200 {
		t.Fatalf("batch status code=%d body=%s", rec.Code, rec.Body.String())
	}
	var batchStatus struct {
		BatchID      string   `json:"batch_id"`
		State        string   `json:"state"`
		EvidenceIDs  []string `json:"evidence_ids"`
		MemberCount  int      `json:"member_count"`
		ManifestHex  string   `json:"manifest_hex"`
		AnchorStatus string   `json:"anchor_status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &batchStatus); err != nil {
		t.Fatal(err)
	}
	if batchStatus.BatchID != frozen.BatchID || batchStatus.State != "pending_submission" || batchStatus.AnchorStatus != "not_submitted" || batchStatus.MemberCount != 1 || batchStatus.ManifestHex != frozen.ManifestHex || len(batchStatus.EvidenceIDs) != 1 || batchStatus.EvidenceIDs[0] != ingested.EvidenceID {
		t.Fatalf("bad batch status: %+v", batchStatus)
	}
	assertNoPrivateMaterial(t, rec.Body.String(), doc)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/evidence/"+ingested.EvidenceID+"?tenant=tenant-a", nil))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), frozen.BatchID) || !strings.Contains(rec.Body.String(), "pending_submission") {
		t.Fatalf("post-freeze status code=%d body=%s", rec.Code, rec.Body.String())
	}
	assertNoPrivateMaterial(t, rec.Body.String(), doc)
	var submitted struct {
		BatchIndex *uint64 `json:"batch_index"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &submitted); err != nil {
		t.Fatal(err)
	}
	if submitted.BatchIndex == nil || *submitted.BatchIndex != 0 {
		t.Fatalf("bad post-freeze batch index: %+v", submitted)
	}
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/outbox?tenant=tenant-a&limit=10", nil))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), frozen.ManifestHex) || !strings.Contains(rec.Body.String(), "pending_submission") {
		t.Fatalf("outbox code=%d body=%s", rec.Code, rec.Body.String())
	}
	var outbox struct {
		Submissions []struct {
			BatchID      string `json:"batch_id"`
			ManifestHex  string `json:"manifest_hex"`
			State        string `json:"state"`
			AnchorStatus string `json:"anchor_status"`
		} `json:"submissions"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &outbox); err != nil {
		t.Fatal(err)
	}
	if len(outbox.Submissions) != 1 || outbox.Submissions[0].BatchID != frozen.BatchID || outbox.Submissions[0].ManifestHex != frozen.ManifestHex || outbox.Submissions[0].AnchorStatus != "not_submitted" {
		t.Fatalf("bad outbox response: %+v", outbox)
	}
	assertNoPrivateMaterial(t, rec.Body.String(), doc)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/evidence/"+ingested.EvidenceID+"/package?tenant=tenant-a", nil))
	if rec.Code != 200 {
		t.Fatalf("export code=%d body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "application/vnd.sigmaproof" {
		t.Fatalf("content-type=%q", got)
	}
	report := proof.Verify(bytes.NewReader(doc), bytes.NewReader(rec.Body.Bytes()), 1024)
	if !report.IntegrityVerified() || report.Anchor.Status != proof.Unavailable {
		t.Fatalf("bad exported package report: %+v", report)
	}
}

func TestEngineHTTPLimitsAndErrors(t *testing.T) {
	s, err := storage.Open(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	handler := HandlerWithStore(s)
	rec := postJSON(t, handler, "/v1/ingest", map[string]any{
		"tenant":          "tenant-a",
		"idempotency_key": "k",
		"source":          map[string]string{"instance": "paperless", "object": "1", "version": "v1"},
		"representation":  "sideways",
		"document_base64": base64.StdEncoding.EncodeToString([]byte("doc")),
	})
	if rec.Code != 400 || !strings.Contains(rec.Body.String(), "invalid_request") {
		t.Fatalf("bad representation code=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = postJSON(t, handler, "/v1/ingest", map[string]any{
		"tenant":          "tenant-a",
		"idempotency_key": "bad-base64",
		"source":          map[string]string{"instance": "paperless", "object": "bad-base64", "version": "v1"},
		"representation":  "original",
		"document_base64": "!!!!",
	})
	if rec.Code != 400 || !strings.Contains(rec.Body.String(), "invalid_request") {
		t.Fatalf("bad base64 code=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/ingest", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "text/plain")
	handler.ServeHTTP(rec, req)
	if rec.Code != 415 || !strings.Contains(rec.Body.String(), "unsupported_media_type") {
		t.Fatalf("bad ingest media type code=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "/v1/ingest", strings.NewReader(`{"tenant":"tenant-a","idempotency_key":"charset","source":{"instance":"paperless","object":"charset","version":"v1"},"representation":"original","document_base64":"ZA=="}`))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	handler.ServeHTTP(rec, req)
	if rec.Code != 201 {
		t.Fatalf("json charset media type code=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/ingest", nil))
	if rec.Code != 405 || rec.Header().Get("Allow") != "POST" || !strings.Contains(rec.Body.String(), "method_not_allowed") {
		t.Fatalf("bad method response code=%d allow=%q body=%s", rec.Code, rec.Header().Get("Allow"), rec.Body.String())
	}
	rec = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "/v1/batches/freeze", strings.NewReader("{}"))
	handler.ServeHTTP(rec, req)
	if rec.Code != 415 || !strings.Contains(rec.Body.String(), "unsupported_media_type") {
		t.Fatalf("missing freeze media type code=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = postJSON(t, handler, "/v1/batches/freeze", map[string]any{"tenant": "tenant-a", "request_key": "batch", "limit": storage.MaxBatchEntries + 1})
	if rec.Code != 400 {
		t.Fatalf("bad freeze limit code=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/outbox?tenant=tenant-a&limit=0", nil))
	if rec.Code != 400 {
		t.Fatalf("bad outbox limit code=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = postJSON(t, handler, "/v1/batches/freeze", map[string]any{"tenant": "tenant-missing", "request_key": "batch", "limit": 1})
	if rec.Code != 404 {
		t.Fatalf("unknown tenant freeze code=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = postJSON(t, handler, "/v1/ingest", map[string]any{
		"tenant":          "tenant-a",
		"idempotency_key": "tenant-a:key",
		"source":          map[string]string{"instance": "paperless", "object": "2", "version": "v1"},
		"representation":  "original",
		"document_base64": base64.StdEncoding.EncodeToString([]byte("doc")),
	})
	if rec.Code != 201 {
		t.Fatalf("setup ingest code=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/batches/00000000000000000000000000000000?tenant=tenant-a", nil))
	if rec.Code != 404 {
		t.Fatalf("unknown batch code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestEngineHTTPConcurrentIngestRetries(t *testing.T) {
	s, err := storage.Open(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	handler := HandlerWithStore(s)
	body := map[string]any{
		"tenant":          "tenant-a",
		"idempotency_key": "paperless:race:v1",
		"source":          map[string]string{"instance": "paperless", "object": "race", "version": "v1"},
		"representation":  "original",
		"document_base64": base64.StdEncoding.EncodeToString([]byte("same uploaded bytes")),
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	createdCount := 0
	ids := map[string]bool{}
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rec := postJSON(t, handler, "/v1/ingest", body)
			if rec.Code != 200 && rec.Code != 201 {
				t.Errorf("ingest code=%d body=%s", rec.Code, rec.Body.String())
				return
			}
			var resp struct {
				EvidenceID string `json:"evidence_id"`
				Created    bool   `json:"created"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Error(err)
				return
			}
			mu.Lock()
			defer mu.Unlock()
			if resp.Created {
				createdCount++
			}
			ids[resp.EvidenceID] = true
		}()
	}
	wg.Wait()
	if createdCount != 1 || len(ids) != 1 {
		t.Fatalf("concurrent HTTP ingest duplicated evidence: created=%d ids=%v", createdCount, ids)
	}
}

func postJSON(t *testing.T, handler http.Handler, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", path, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(rec, req)
	return rec
}

func assertNoPrivateMaterial(t *testing.T, body string, document []byte) {
	t.Helper()
	digest := sha256.Sum256(document)
	for _, private := range []string{
		"document_digest",
		"nonce",
		base64.StdEncoding.EncodeToString(document),
		hex.EncodeToString(document),
		hex.EncodeToString(digest[:]),
	} {
		if strings.Contains(body, private) {
			t.Fatalf("response exposed private material %q in %s", private, body)
		}
	}
}
