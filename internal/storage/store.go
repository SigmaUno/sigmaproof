// Package storage implements the private, single-process transactional MVP store.
// Tenant arguments must come from an authenticated caller; this is not an auth API.
package storage

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"time"
	"unicode/utf8"

	bolt "go.etcd.io/bbolt"

	"github.com/SigmaUno/sigmaproof/pkg/proof"
	"github.com/SigmaUno/sigmaproof/pkg/proof/batch"
	"github.com/SigmaUno/sigmaproof/pkg/proof/commitment"
	"github.com/SigmaUno/sigmaproof/pkg/proof/merkle"
)

const MaxBatchEntries = 1024
const maxRecordBytes = 128 << 10

const (
	SubmissionNotSubmitted = "not_submitted"
	SubmissionUnknown      = "unknown"
	SubmissionSubmitted    = "submitted"
)

var (
	ErrConflict   = errors.New("idempotency or source-version conflict")
	ErrNotFound   = errors.New("record not found")
	ErrNotBatched = errors.New("evidence has no frozen batch")
	ErrEmpty      = errors.New("no pending evidence")
	ErrInvalid    = errors.New("invalid storage input")
	ErrCorrupt    = errors.New("inconsistent storage record")
)

var tenantBuckets = []string{"evidence", "keys", "sources", "pending", "batches", "batch_keys", "outbox"}

type Store struct{ db *bolt.DB }

type Source struct{ Instance, Object, Version string }

type IngestRequest struct {
	Tenant  string
	Key     string
	Source  Source
	Witness commitment.Witness
}

type DocumentIngestRequest struct {
	Tenant         string
	Key            string
	Source         Source
	Representation commitment.Representation
	Document       io.Reader
	MaxBytes       int64
}

// Evidence and FrozenBatch are private storage records, not public anchor payloads.
type Evidence struct {
	ID       string
	Sequence uint64
	Source   Source
	Witness  commitment.Witness
	BatchID  string
	Index    uint64
}

type FrozenBatch struct {
	ID              string
	Limit           int
	EvidenceIDs     []string
	Manifest        []byte
	SubmissionState string
	SubmissionRef   string
}

// Submission exposes only pending exact public manifests plus opaque local batch IDs.
// Unknown outcomes stay pending; submitted batches leave the outbox.
type Submission struct {
	BatchID  string
	Manifest []byte
	State    string
	Ref      string
}

// Open opens a mode-0600 database on a local filesystem with sync enabled and an
// exclusive process lock. Unknown schemas/foreign databases are rejected unchanged.
func Open(path string) (*Store, error) {
	if path == "" {
		return nil, ErrInvalid
	}
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
			return nil, errors.New("database must be a private regular file")
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	db, err := bolt.Open(path, 0600, &bolt.Options{Timeout: time.Second})
	if err != nil {
		return nil, err
	}
	err = db.Update(func(tx *bolt.Tx) error {
		meta := tx.Bucket([]byte("schema"))
		if meta != nil {
			if !bytes.Equal(meta.Get([]byte("version")), []byte{1}) || tx.Bucket([]byte("tenants")) == nil {
				return errors.New("unsupported storage schema")
			}
			return nil
		}
		key, _ := tx.Cursor().First()
		if key != nil {
			return errors.New("refusing to initialize foreign database")
		}
		meta, err := tx.CreateBucket([]byte("schema"))
		if err != nil {
			return err
		}
		if err := meta.Put([]byte("version"), []byte{1}); err != nil {
			return err
		}
		_, err = tx.CreateBucket([]byte("tenants"))
		return err
	})
	if err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func validText(value string) bool {
	return len(value) > 0 && len(value) <= 256 && utf8.ValidString(value) && !bytes.ContainsRune([]byte(value), 0)
}
func validID(id string) bool {
	if len(id) != 32 {
		return false
	}
	_, err := hex.DecodeString(id)
	return err == nil
}
func validSource(source Source) bool {
	return validText(source.Instance) && validText(source.Object) && validText(source.Version)
}
func sequenceKey(n uint64) []byte {
	out := make([]byte, 8)
	binary.BigEndian.PutUint64(out, n)
	return out
}
func sourceKey(source Source, rep commitment.Representation) []byte {
	data, _ := json.Marshal(struct {
		Source         Source
		Representation commitment.Representation
	}{source, rep})
	return data // Internal framing only; never a public protocol encoding.
}
func newID(bucket *bolt.Bucket) (string, error) {
	for {
		var raw [16]byte
		if _, err := rand.Read(raw[:]); err != nil {
			return "", err
		}
		id := hex.EncodeToString(raw[:])
		if bucket.Get([]byte(id)) == nil {
			return id, nil
		}
	}
}
func put(bucket *bolt.Bucket, key []byte, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return bucket.Put(key, data)
}
func decode(data []byte, target any) error {
	if data == nil {
		return ErrNotFound
	}
	if len(data) > maxRecordBytes {
		return ErrCorrupt
	}
	if err := json.Unmarshal(data, target); err != nil {
		return ErrCorrupt
	}
	return nil
}
func tenant(tx *bolt.Tx, name string, create bool) (*bolt.Bucket, error) {
	tenants := tx.Bucket([]byte("tenants"))
	if tenants == nil {
		return nil, ErrCorrupt
	}
	t := tenants.Bucket([]byte(name))
	if t == nil {
		if !create {
			return nil, ErrNotFound
		}
		var err error
		t, err = tenants.CreateBucket([]byte(name))
		if err != nil {
			return nil, err
		}
		for _, name := range tenantBuckets {
			if _, err := t.CreateBucket([]byte(name)); err != nil {
				return nil, err
			}
		}
	}
	for _, name := range tenantBuckets {
		if t.Bucket([]byte(name)) == nil {
			return nil, ErrCorrupt
		}
	}
	return t, nil
}
func evidence(t *bolt.Bucket, id string) (Evidence, error) {
	var e Evidence
	if err := decode(t.Bucket([]byte("evidence")).Get([]byte(id)), &e); err != nil {
		return Evidence{}, err
	}
	if e.ID != id || !validID(id) || e.Sequence == 0 || !validSource(e.Source) {
		return Evidence{}, ErrCorrupt
	}
	if _, err := commitment.Compute(e.Witness); err != nil {
		return Evidence{}, ErrCorrupt
	}
	return e, nil
}
func frozen(t *bolt.Bucket, id string) (FrozenBatch, error) {
	var b FrozenBatch
	if err := decode(t.Bucket([]byte("batches")).Get([]byte(id)), &b); err != nil {
		return FrozenBatch{}, err
	}
	if b.SubmissionState == "" {
		b.SubmissionState = SubmissionNotSubmitted
	}
	m, err := batch.Parse(b.Manifest)
	if err != nil || b.ID != id || !validID(id) || b.Limit < 1 || b.Limit > MaxBatchEntries || len(b.EvidenceIDs) > b.Limit || uint64(len(b.EvidenceIDs)) != m.Size || !validSubmissionState(b.SubmissionState) || (b.SubmissionRef != "" && !validText(b.SubmissionRef)) {
		return FrozenBatch{}, ErrCorrupt
	}
	return b, nil
}

func validSubmissionState(state string) bool {
	switch state {
	case SubmissionNotSubmitted, SubmissionUnknown, SubmissionSubmitted:
		return true
	default:
		return false
	}
}

// Ingest atomically persists the witness, idempotency/source indexes and pending
// entry. Identical retries return the original record; changed witnesses conflict.
// The bool is true only for a newly committed evidence record.
func (s *Store) Ingest(req IngestRequest) (Evidence, bool, error) {
	if !validText(req.Tenant) || !validText(req.Key) || !validSource(req.Source) {
		return Evidence{}, false, ErrInvalid
	}
	if _, err := commitment.Compute(req.Witness); err != nil {
		return Evidence{}, false, ErrInvalid
	}
	var result Evidence
	var created bool
	err := s.db.Update(func(tx *bolt.Tx) error {
		var err error
		result, created, err = ingest(tx, req)
		return err
	})
	if err != nil {
		return Evidence{}, false, err
	}
	return result, created, nil
}

// IngestDocument hashes document bytes and creates a fresh witness only when no
// matching idempotency/source record exists. Exact retries reuse the original
// nonce and verify that the supplied document still matches it.
func (s *Store) IngestDocument(req DocumentIngestRequest) (Evidence, bool, error) {
	if !validText(req.Tenant) || !validText(req.Key) || !validSource(req.Source) || (req.Representation != commitment.Original && req.Representation != commitment.Derived) || req.Document == nil || req.MaxBytes < 0 || req.MaxBytes == math.MaxInt64 {
		return Evidence{}, false, ErrInvalid
	}
	existing, found, err := s.find(req.Tenant, req.Key, req.Source, req.Representation)
	if err != nil {
		return Evidence{}, false, err
	}
	if found {
		c, err := commitment.Compute(existing.Witness)
		if err != nil {
			return Evidence{}, false, ErrCorrupt
		}
		if err := commitment.Verify(req.Document, existing.Witness, c, req.MaxBytes); err != nil {
			return Evidence{}, false, ErrConflict
		}
		return existing, false, nil
	}
	w, _, err := commitment.New(req.Document, req.Representation, req.MaxBytes)
	if err != nil {
		return Evidence{}, false, ErrInvalid
	}
	result, created, err := s.Ingest(IngestRequest{Tenant: req.Tenant, Key: req.Key, Source: req.Source, Witness: w})
	if errors.Is(err, ErrConflict) {
		existing, found, findErr := s.find(req.Tenant, req.Key, req.Source, req.Representation)
		if findErr != nil {
			return Evidence{}, false, findErr
		}
		if found && existing.Witness.DocumentDigest == w.DocumentDigest {
			return existing, false, nil
		}
		return Evidence{}, false, ErrConflict
	}
	return result, created, err
}

func (s *Store) find(tenantID, key string, source Source, rep commitment.Representation) (Evidence, bool, error) {
	var result Evidence
	err := s.db.View(func(tx *bolt.Tx) error {
		t, err := tenant(tx, tenantID, false)
		if errors.Is(err, ErrNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		id := t.Bucket([]byte("keys")).Get([]byte(key))
		if id == nil {
			id = t.Bucket([]byte("sources")).Get(sourceKey(source, rep))
		}
		if id == nil {
			return nil
		}
		result, err = evidence(t, string(id))
		return err
	})
	if err != nil {
		return Evidence{}, false, err
	}
	if result.ID == "" {
		return Evidence{}, false, nil
	}
	if result.Source != source || result.Witness.Representation != rep {
		return Evidence{}, false, ErrConflict
	}
	return result, true, nil
}

func ingest(tx *bolt.Tx, req IngestRequest) (Evidence, bool, error) {
	t, err := tenant(tx, req.Tenant, true)
	if err != nil {
		return Evidence{}, false, err
	}
	keys, sources := t.Bucket([]byte("keys")), t.Bucket([]byte("sources"))
	sk := sourceKey(req.Source, req.Witness.Representation)
	id := keys.Get([]byte(req.Key))
	if id == nil {
		id = sources.Get(sk)
	}
	if id != nil {
		e, err := evidence(t, string(id))
		if err != nil {
			return Evidence{}, false, err
		}
		if e.Source != req.Source || e.Witness != req.Witness {
			return Evidence{}, false, ErrConflict
		}
		if err := keys.Put([]byte(req.Key), []byte(e.ID)); err != nil {
			return Evidence{}, false, err
		}
		return e, false, nil
	}
	records := t.Bucket([]byte("evidence"))
	idString, err := newID(records)
	if err != nil {
		return Evidence{}, false, err
	}
	sequence, err := records.NextSequence()
	if err != nil || sequence == 0 {
		return Evidence{}, false, ErrCorrupt
	}
	e := Evidence{ID: idString, Sequence: sequence, Source: req.Source, Witness: req.Witness}
	if err := put(records, []byte(e.ID), e); err != nil {
		return Evidence{}, false, err
	}
	for _, write := range []struct {
		bucket     *bolt.Bucket
		key, value []byte
	}{
		{keys, []byte(req.Key), []byte(e.ID)}, {sources, sk, []byte(e.ID)},
		{t.Bucket([]byte("pending")), sequenceKey(sequence), []byte(e.ID)},
	} {
		if err := write.bucket.Put(write.key, write.value); err != nil {
			return Evidence{}, false, err
		}
	}
	return e, true, nil
}

func (s *Store) Evidence(tenantID, id string) (Evidence, error) {
	if !validText(tenantID) || !validID(id) {
		return Evidence{}, ErrInvalid
	}
	var result Evidence
	err := s.db.View(func(tx *bolt.Tx) error {
		t, err := tenant(tx, tenantID, false)
		if err != nil {
			return err
		}
		result, err = evidence(t, id)
		return err
	})
	if err != nil {
		return Evidence{}, err
	}
	return result, nil
}

func (s *Store) Batch(tenantID, id string) (FrozenBatch, error) {
	if !validText(tenantID) || !validID(id) {
		return FrozenBatch{}, ErrInvalid
	}
	var result FrozenBatch
	err := s.db.View(func(tx *bolt.Tx) error {
		t, err := tenant(tx, tenantID, false)
		if err != nil {
			return err
		}
		result, err = frozen(t, id)
		return err
	})
	if err != nil {
		return FrozenBatch{}, err
	}
	result.Manifest = bytes.Clone(result.Manifest)
	result.EvidenceIDs = append([]string(nil), result.EvidenceIDs...)
	return result, nil
}

// RecordSubmission durably records local submission progress for a frozen batch.
// Unknown outcomes remain in the outbox for reconciliation or retry. Submitted
// batches are removed from pending work, but this is not proof authentication.
func (s *Store) RecordSubmission(tenantID, batchID, state, reference string) (FrozenBatch, error) {
	if !validText(tenantID) || !validID(batchID) || !validSubmissionState(state) || (reference != "" && !validText(reference)) {
		return FrozenBatch{}, ErrInvalid
	}
	var result FrozenBatch
	err := s.db.Update(func(tx *bolt.Tx) error {
		t, err := tenant(tx, tenantID, false)
		if err != nil {
			return err
		}
		result, err = frozen(t, batchID)
		if err != nil {
			return err
		}
		if result.SubmissionState == SubmissionSubmitted && state != SubmissionSubmitted {
			return ErrConflict
		}
		result.SubmissionState = state
		result.SubmissionRef = reference
		if err := put(t.Bucket([]byte("batches")), []byte(batchID), result); err != nil {
			return err
		}
		outbox := t.Bucket([]byte("outbox"))
		if state == SubmissionSubmitted {
			return outbox.Delete([]byte(batchID))
		}
		return outbox.Put([]byte(batchID), result.Manifest)
	})
	if err != nil {
		return FrozenBatch{}, err
	}
	result.Manifest = bytes.Clone(result.Manifest)
	result.EvidenceIDs = append([]string(nil), result.EvidenceIDs...)
	return result, nil
}

// Freeze atomically selects FIFO pending evidence, freezes exact manifest bytes,
// binds each member and creates an outbox entry. A request key retries the same
// batch even if new evidence arrived. Changed limits for the same key conflict.
func (s *Store) Freeze(tenantID, key string, limit int) (FrozenBatch, error) {
	if !validText(tenantID) || !validText(key) || limit < 1 || limit > MaxBatchEntries {
		return FrozenBatch{}, ErrInvalid
	}
	var result FrozenBatch
	err := s.db.Update(func(tx *bolt.Tx) error {
		var err error
		result, err = freeze(tx, tenantID, key, limit)
		return err
	})
	if err != nil {
		return FrozenBatch{}, err
	}
	return result, nil
}

func freeze(tx *bolt.Tx, tenantID, key string, limit int) (FrozenBatch, error) {
	t, err := tenant(tx, tenantID, false)
	if err != nil {
		return FrozenBatch{}, err
	}
	keys := t.Bucket([]byte("batch_keys"))
	if id := keys.Get([]byte(key)); id != nil {
		b, err := frozen(t, string(id))
		if err != nil {
			return FrozenBatch{}, err
		}
		if b.Limit != limit {
			return FrozenBatch{}, ErrConflict
		}
		return b, nil
	}
	pending := t.Bucket([]byte("pending"))
	cursor := pending.Cursor()
	var members []Evidence
	var commitments []commitment.Digest
	for k, id := cursor.First(); k != nil && len(members) < limit; k, id = cursor.Next() {
		e, err := evidence(t, string(id))
		if err != nil {
			return FrozenBatch{}, err
		}
		if !bytes.Equal(k, sequenceKey(e.Sequence)) || e.BatchID != "" {
			return FrozenBatch{}, ErrCorrupt
		}
		c, err := commitment.Compute(e.Witness)
		if err != nil {
			return FrozenBatch{}, ErrCorrupt
		}
		members = append(members, e)
		commitments = append(commitments, c)
	}
	if len(members) == 0 {
		return FrozenBatch{}, ErrEmpty
	}
	m, err := batch.New(commitments)
	if err != nil {
		return FrozenBatch{}, err
	}
	manifest, err := m.MarshalBinary()
	if err != nil {
		return FrozenBatch{}, err
	}
	id, err := newID(t.Bucket([]byte("batches")))
	if err != nil {
		return FrozenBatch{}, err
	}
	b := FrozenBatch{ID: id, Limit: limit, Manifest: manifest, SubmissionState: SubmissionNotSubmitted}
	for i, e := range members {
		e.BatchID, e.Index = id, uint64(i)
		if err := put(t.Bucket([]byte("evidence")), []byte(e.ID), e); err != nil {
			return FrozenBatch{}, err
		}
		if err := pending.Delete(sequenceKey(e.Sequence)); err != nil {
			return FrozenBatch{}, err
		}
		b.EvidenceIDs = append(b.EvidenceIDs, e.ID)
	}
	if err := put(t.Bucket([]byte("batches")), []byte(id), b); err != nil {
		return FrozenBatch{}, err
	}
	if err := keys.Put([]byte(key), []byte(id)); err != nil {
		return FrozenBatch{}, err
	}
	if err := t.Bucket([]byte("outbox")).Put([]byte(id), manifest); err != nil {
		return FrozenBatch{}, err
	}
	return b, nil
}

// Outbox lists pending immutable manifests. It does not claim or submit work.
// Unknown submission outcomes remain visible until resolved or retried.
func (s *Store) Outbox(tenantID string, limit int) ([]Submission, error) {
	if !validText(tenantID) || limit < 1 || limit > MaxBatchEntries {
		return nil, ErrInvalid
	}
	result := []Submission{}
	err := s.db.View(func(tx *bolt.Tx) error {
		t, err := tenant(tx, tenantID, false)
		if err != nil {
			return err
		}
		c := t.Bucket([]byte("outbox")).Cursor()
		for id, payload := c.First(); id != nil && len(result) < limit; id, payload = c.Next() {
			b, err := frozen(t, string(id))
			if err != nil {
				return err
			}
			if !bytes.Equal(payload, b.Manifest) {
				return ErrCorrupt
			}
			result = append(result, Submission{BatchID: string(id), Manifest: bytes.Clone(payload), State: b.SubmissionState, Ref: b.SubmissionRef})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// UnanchoredPackage reconstructs local evidence for a frozen member from durable
// records. It never labels pending outbox work as authenticated publication.
func (s *Store) UnanchoredPackage(tenantID, id string) (proof.Package, error) {
	if !validText(tenantID) || !validID(id) {
		return proof.Package{}, ErrInvalid
	}
	var result proof.Package
	err := s.db.View(func(tx *bolt.Tx) error {
		t, err := tenant(tx, tenantID, false)
		if err != nil {
			return err
		}
		e, err := evidence(t, id)
		if err != nil {
			return err
		}
		if e.BatchID == "" {
			return ErrNotBatched
		}
		b, err := frozen(t, e.BatchID)
		if err != nil {
			return err
		}
		if e.Index >= uint64(len(b.EvidenceIDs)) || b.EvidenceIDs[e.Index] != id {
			return ErrCorrupt
		}
		entries := make([][]byte, len(b.EvidenceIDs))
		for i, memberID := range b.EvidenceIDs {
			member, err := evidence(t, memberID)
			if err != nil {
				return err
			}
			if member.BatchID != b.ID || member.Index != uint64(i) {
				return ErrCorrupt
			}
			c, err := commitment.Compute(member.Witness)
			if err != nil {
				return ErrCorrupt
			}
			entries[i] = c[:]
		}
		m, err := batch.Parse(b.Manifest)
		if err != nil {
			return ErrCorrupt
		}
		if merkle.Root(entries) != m.Root {
			return ErrCorrupt
		}
		path, err := merkle.Path(entries, e.Index)
		if err != nil {
			return err
		}
		result = proof.Package{Witness: e.Witness, Manifest: m, Index: e.Index, Path: path}
		return nil
	})
	if err != nil {
		return proof.Package{}, err
	}
	return result, nil
}
