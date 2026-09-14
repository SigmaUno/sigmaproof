package agent

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWebhookAuthenticatesAndIngests(t *testing.T) {
	store := openAgentStore(t)
	webhook := Webhook{
		Runner: Runner{
			Fetcher:  &memoryFetcher{version: "v1", doc: []byte("document")},
			Ingester: store,
			MaxBytes: 1024,
		},
		Secret: []byte("shared-secret"),
	}
	body := webhookBody(t, WebhookPayload{Tenant: "tenant-a", Instance: "paperless-main", DocumentID: "42", Version: "v1", EventID: "event-1", Representation: string(OriginalUpload)})
	req := signedRequest(body, webhook.Secret, DefaultSignatureHeader)
	result, err := webhook.Handle(req)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Created || result.EvidenceID == "" {
		t.Fatalf("webhook did not ingest: %+v", result)
	}
	retry, err := webhook.Handle(signedRequest(body, webhook.Secret, DefaultSignatureHeader))
	if err != nil {
		t.Fatal(err)
	}
	if retry.Created || retry.EvidenceID != result.EvidenceID {
		t.Fatalf("webhook retry not idempotent: %+v", retry)
	}
}

func TestWebhookServeHTTPResponses(t *testing.T) {
	store := openAgentStore(t)
	webhook := Webhook{
		Runner: Runner{
			Fetcher:        &memoryFetcher{version: "v1", doc: []byte("document")},
			Ingester:       store,
			EvidenceFields: []string{"sigmaproof_status"},
		},
		Secret: []byte("shared-secret"),
	}
	body := webhookBody(t, WebhookPayload{ChangedFields: []string{"sigmaproof_status"}})
	rec := httptest.NewRecorder()
	webhook.ServeHTTP(rec, signedRequest(body, webhook.Secret, DefaultSignatureHeader))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"skipped":true`) {
		t.Fatalf("bad skipped response: %d %s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/paperless/webhook", nil)
	webhook.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != http.MethodPost {
		t.Fatalf("bad method response: %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	bad := httptest.NewRequest(http.MethodPost, "/paperless/webhook", bytes.NewReader(body))
	bad.Header.Set(DefaultSignatureHeader, "sha256=bad")
	webhook.ServeHTTP(rec, bad)
	if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), "invalid_signature") {
		t.Fatalf("bad signature response: %d %s", rec.Code, rec.Body.String())
	}
}

func TestWebhookRejectsMalformedBodiesAndConfig(t *testing.T) {
	webhook := Webhook{Secret: []byte("shared-secret"), MaxBodyBytes: 8}
	body := []byte(`{"tenant":"tenant-a"}`)
	if _, err := webhook.Handle(signedRequest(body, webhook.Secret, DefaultSignatureHeader)); err == nil {
		t.Fatal("oversized body accepted")
	}
	webhook.MaxBodyBytes = 0
	if _, err := webhook.Handle(signedRequest([]byte(`{"unknown":1}`), webhook.Secret, DefaultSignatureHeader)); err == nil {
		t.Fatal("unknown field accepted")
	}
	webhook.Secret = nil
	if _, err := webhook.Handle(signedRequest([]byte(`{}`), []byte("shared-secret"), DefaultSignatureHeader)); err == nil {
		t.Fatal("missing secret accepted")
	}
}

func TestWebhookCustomSignatureHeader(t *testing.T) {
	store := openAgentStore(t)
	webhook := Webhook{
		Runner:          Runner{Fetcher: &memoryFetcher{version: "v1", doc: []byte("document")}, Ingester: store},
		Secret:          []byte("shared-secret"),
		SignatureHeader: "X-Paperless-Signature",
	}
	body := webhookBody(t, WebhookPayload{Tenant: "tenant-a", Instance: "paperless-main", DocumentID: "42", Version: "v1", EventID: "event-1", Representation: string(ArchiveOutput)})
	if _, err := webhook.Handle(signedRequest(body, webhook.Secret, "X-Paperless-Signature")); err != nil {
		t.Fatal(err)
	}
}

func webhookBody(t *testing.T, payload WebhookPayload) []byte {
	t.Helper()
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func signedRequest(body, secret []byte, header string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/paperless/webhook", bytes.NewReader(body))
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	req.Header.Set(header, "sha256="+hex.EncodeToString(mac.Sum(nil)))
	return req
}
