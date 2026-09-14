package agent

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"github.com/SigmaUno/sigmaproof/internal/storage"
)

const DefaultMaxDocumentBytes = 64 << 20

type FetchRequest struct {
	Instance       string
	DocumentID     string
	Version        string
	Representation Representation
}

type FetchResult struct {
	Version  string
	Document io.Reader
}

type Fetcher interface {
	Fetch(FetchRequest) (FetchResult, error)
}

type Ingester interface {
	IngestDocument(storage.DocumentIngestRequest) (storage.Evidence, bool, error)
}

type Runner struct {
	Fetcher        Fetcher
	Ingester       Ingester
	EvidenceFields []string
	MaxBytes       int64
}

type Result struct {
	Plan       Plan
	EvidenceID string
	Created    bool
	Skipped    bool
}

var (
	ErrNotConfigured   = errors.New("paperless runner is not configured")
	ErrVersionChanged  = errors.New("paperless representation changed while fetching")
	ErrMissingDocument = errors.New("paperless fetch returned no document")
	ErrOversizedFetch  = errors.New("paperless fetch exceeded byte limit")
)

// Handle plans an authenticated Paperless event, fetches the exact selected
// bytes and submits them through the durable ingestion boundary.
func (r Runner) Handle(event Event) (Result, error) {
	plan, err := PlanIngest(event, r.EvidenceFields)
	if err != nil {
		return Result{}, err
	}
	if !plan.Ingest {
		return Result{Plan: plan, Skipped: true}, nil
	}
	if r.Fetcher == nil || r.Ingester == nil {
		return Result{}, ErrNotConfigured
	}
	maxBytes := r.MaxBytes
	if maxBytes == 0 {
		maxBytes = DefaultMaxDocumentBytes
	}
	if maxBytes < 0 {
		return Result{}, ErrNotConfigured
	}
	fetched, err := r.Fetcher.Fetch(FetchRequest{
		Instance:       event.Instance,
		DocumentID:     event.DocumentID,
		Version:        event.Version,
		Representation: event.Representation,
	})
	if err != nil {
		return Result{}, err
	}
	if fetched.Version != "" && fetched.Version != event.Version {
		return Result{}, ErrVersionChanged
	}
	if fetched.Document == nil {
		return Result{}, ErrMissingDocument
	}
	document, err := boundedBytes(fetched.Document, maxBytes)
	if err != nil {
		return Result{}, err
	}
	evidence, created, err := r.Ingester.IngestDocument(storage.DocumentIngestRequest{
		Tenant:         plan.Tenant,
		Key:            plan.IdempotencyKey,
		Source:         plan.Source,
		Representation: plan.Representation,
		Document:       bytes.NewReader(document),
		MaxBytes:       maxBytes,
	})
	if err != nil {
		return Result{}, err
	}
	return Result{Plan: plan, EvidenceID: evidence.ID, Created: created}, nil
}

func boundedBytes(r io.Reader, maxBytes int64) ([]byte, error) {
	limited := io.LimitReader(r, maxBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("%w: max %d bytes", ErrOversizedFetch, maxBytes)
	}
	return data, nil
}
