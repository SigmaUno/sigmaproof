package agent

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
)

const (
	DefaultSignatureHeader = "X-SigmaProof-Signature"
	DefaultWebhookMaxBytes = 64 << 10
)

type Webhook struct {
	Runner          Runner
	Secret          []byte
	SignatureHeader string
	MaxBodyBytes    int64
}

type WebhookPayload struct {
	Tenant         string   `json:"tenant"`
	Instance       string   `json:"instance"`
	DocumentID     string   `json:"document_id"`
	Version        string   `json:"version"`
	EventID        string   `json:"event_id"`
	Representation string   `json:"representation"`
	ChangedFields  []string `json:"changed_fields"`
}

var (
	ErrWebhookNotConfigured = errors.New("paperless webhook is not configured")
	ErrWebhookSignature     = errors.New("invalid paperless webhook signature")
	ErrWebhookBody          = errors.New("invalid paperless webhook body")
)

// ServeHTTP verifies a bounded JSON workflow webhook and ingests it through the
// same runner path used by reconciliation. The shared secret is supplied by the
// operator as a custom Paperless workflow header.
func (w Webhook) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		rw.Header().Set("Allow", http.MethodPost)
		writeWebhookJSON(rw, http.StatusMethodNotAllowed, map[string]string{"error": "method_not_allowed"})
		return
	}
	result, err := w.Handle(r)
	if err != nil {
		status := http.StatusBadRequest
		code := "invalid_request"
		if errors.Is(err, ErrWebhookNotConfigured) {
			status, code = http.StatusInternalServerError, "not_configured"
		} else if errors.Is(err, ErrWebhookSignature) {
			status, code = http.StatusUnauthorized, "invalid_signature"
		}
		writeWebhookJSON(rw, status, map[string]string{"error": code})
		return
	}
	status := http.StatusOK
	if result.Created {
		status = http.StatusCreated
	}
	writeWebhookJSON(rw, status, map[string]any{
		"evidence_id": result.EvidenceID,
		"created":     result.Created,
		"skipped":     result.Skipped,
		"reason":      result.Plan.Reason,
	})
}

func (w Webhook) Handle(r *http.Request) (Result, error) {
	if len(w.Secret) == 0 {
		return Result{}, ErrWebhookNotConfigured
	}
	maxBytes := w.MaxBodyBytes
	if maxBytes == 0 {
		maxBytes = DefaultWebhookMaxBytes
	}
	if maxBytes < 0 {
		return Result{}, ErrWebhookNotConfigured
	}
	body, err := io.ReadAll(http.MaxBytesReader(nil, r.Body, maxBytes))
	if err != nil {
		return Result{}, ErrWebhookBody
	}
	if !validSignature(signatureHeader(w.SignatureHeader), r.Header, body, w.Secret) {
		return Result{}, ErrWebhookSignature
	}
	var payload WebhookPayload
	dec := json.NewDecoder(strings.NewReader(string(body)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&payload); err != nil {
		return Result{}, ErrWebhookBody
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return Result{}, ErrWebhookBody
	}
	return w.Runner.Handle(payload.event())
}

func (p WebhookPayload) event() Event {
	return Event{
		Tenant:         p.Tenant,
		Instance:       p.Instance,
		DocumentID:     p.DocumentID,
		Version:        p.Version,
		EventID:        p.EventID,
		Representation: Representation(p.Representation),
		ChangedFields:  append([]string(nil), p.ChangedFields...),
	}
}

func validSignature(header string, h http.Header, body, secret []byte) bool {
	got := strings.TrimSpace(h.Get(header))
	if got == "" {
		return false
	}
	got = strings.TrimPrefix(got, "sha256=")
	signature, err := hex.DecodeString(got)
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	return hmac.Equal(signature, mac.Sum(nil))
}

func signatureHeader(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return DefaultSignatureHeader
	}
	return value
}

func writeWebhookJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
