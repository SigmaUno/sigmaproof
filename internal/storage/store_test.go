package storage

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	bolt "go.etcd.io/bbolt"

	"github.com/SigmaUno/sigmaproof/pkg/proof"
	"github.com/SigmaUno/sigmaproof/pkg/proof/batch"
	"github.com/SigmaUno/sigmaproof/pkg/proof/commitment"
)

func request(n int) (IngestRequest, []byte) {
	doc := []byte(fmt.Sprintf("synthetic document %d", n))
	w := commitment.Witness{Version: 1, Algorithm: 1, Representation: commitment.Original, DocumentDigest: sha256.Sum256(doc), Nonce: sha256.Sum256([]byte(fmt.Sprintf("synthetic test nonce %d", n)))}
	return IngestRequest{Tenant: "tenant-a", Key: fmt.Sprintf("key-%d", n), Source: Source{"paperless", fmt.Sprintf("object-%d", n), "version-1"}, Witness: w}, doc
}
func openTest(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestIdempotencyAndSourceIdentity(t *testing.T) {
	s := openTest(t)
	req, _ := request(1)
	first, created, err := s.Ingest(req)
	if err != nil || !created {
		t.Fatalf("ingest: %v", err)
	}
	retry, created, err := s.Ingest(req)
	if err != nil || created || retry != first {
		t.Fatalf("retry changed evidence: %v", err)
	}
	alias := req
	alias.Key = "another-key"
	retry, created, err = s.Ingest(alias)
	if err != nil || created || retry != first {
		t.Fatal("source deduplication failed")
	}
	for _, mutate := range []func(*IngestRequest){
		func(r *IngestRequest) { r.Witness.Nonce[0] ^= 1 },
		func(r *IngestRequest) { r.Witness.DocumentDigest[0] ^= 1 },
		func(r *IngestRequest) { r.Source.Version = "changed" },
		func(r *IngestRequest) { r.Source.Object = "changed" },
		func(r *IngestRequest) { r.Witness.Representation = commitment.Derived },
	} {
		changed := req
		mutate(&changed)
		e, created, err := s.Ingest(changed)
		if !errors.Is(err, ErrConflict) || created || e != (Evidence{}) {
			t.Fatalf("conflicting retry accepted: %v", err)
		}
	}
	changedNonce := req
	changedNonce.Key = "new-key"
	changedNonce.Witness.Nonce[0] ^= 1
	if _, _, err := s.Ingest(changedNonce); !errors.Is(err, ErrConflict) {
		t.Fatal("new key bypassed source conflict")
	}
	newVersion := req
	newVersion.Key = "new-version-key"
	newVersion.Source.Version = "version-2"
	if _, created, err := s.Ingest(newVersion); err != nil || !created {
		t.Fatal("distinct source version not accepted")
	}
}

func TestFreezeFIFOExportAndIsolation(t *testing.T) {
	s := openTest(t)
	var ids []string
	var docs [][]byte
	for i := 0; i < 5; i++ {
		req, doc := request(i)
		e, _, err := s.Ingest(req)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, e.ID)
		docs = append(docs, doc)
	}
	if _, err := s.UnanchoredPackage("tenant-a", ids[0]); !errors.Is(err, ErrNotBatched) {
		t.Fatal("export before freeze succeeded")
	}
	b, err := s.Freeze("tenant-a", "batch-1", 3)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(b.EvidenceIDs, ids[:3]) {
		t.Fatal("batch order is not FIFO")
	}
	next, err := s.Freeze("tenant-a", "batch-2", 3)
	if err != nil || !reflect.DeepEqual(next.EvidenceIDs, ids[3:]) {
		t.Fatal("remaining evidence not frozen correctly")
	}
	again, err := s.Freeze("tenant-a", "batch-1", 3)
	if err != nil || !reflect.DeepEqual(again, b) {
		t.Fatal("retry changed batch")
	}
	if _, err := s.Freeze("tenant-a", "batch-1", 2); !errors.Is(err, ErrConflict) {
		t.Fatal("changed freeze parameters accepted")
	}
	if _, err := s.Freeze("tenant-a", "batch-3", 3); !errors.Is(err, ErrEmpty) {
		t.Fatal("empty batch created")
	}
	for i, id := range ids {
		p, err := s.UnanchoredPackage("tenant-a", id)
		if err != nil {
			t.Fatal(err)
		}
		data, err := p.MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}
		report := proof.Verify(bytes.NewReader(docs[i]), bytes.NewReader(data), 1024)
		if !report.IntegrityVerified() || report.Anchor.Status != proof.Unavailable {
			t.Fatalf("bad export report: %+v", report)
		}
		if _, err := s.Evidence("tenant-b", id); !errors.Is(err, ErrNotFound) {
			t.Fatal("cross-tenant evidence access")
		}
		if _, err := s.UnanchoredPackage("tenant-b", id); !errors.Is(err, ErrNotFound) {
			t.Fatal("cross-tenant export")
		}
	}
	outbox, err := s.Outbox("tenant-a", 10)
	if err != nil || len(outbox) != 2 {
		t.Fatal("wrong pending outbox")
	}
	for _, item := range outbox {
		if _, err := batch.Parse(item.Manifest); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 5; i++ {
			req, _ := request(i)
			for _, private := range [][]byte{docs[i], req.Witness.Nonce[:], req.Witness.DocumentDigest[:]} {
				if bytes.Contains(item.Manifest, private) {
					t.Fatal("private data in public payload")
				}
			}
		}
	}
	// Mutating returned bytes does not mutate the database.
	outbox[0].Manifest[0] ^= 1
	b.Manifest[0] ^= 1
	if _, err := s.Outbox("tenant-a", 10); err != nil {
		t.Fatal("returned buffer aliases database")
	}
	storedBatch, err := s.Batch("tenant-a", b.ID)
	if err != nil || !reflect.DeepEqual(storedBatch.EvidenceIDs, ids[:3]) {
		t.Fatal("batch lookup failed")
	}
	storedBatch.Manifest[0] ^= 1
	storedBatch.EvidenceIDs[0] = ids[3]
	storedBatch, err = s.Batch("tenant-a", b.ID)
	if err != nil || !reflect.DeepEqual(storedBatch.EvidenceIDs, ids[:3]) {
		t.Fatal("returned batch aliases database")
	}
	if _, err := s.Batch("tenant-b", b.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("cross-tenant batch access")
	}
	req, _ := request(0)
	req.Tenant = "tenant-b"
	foreign, created, err := s.Ingest(req)
	if err != nil || !created || foreign.ID == ids[0] {
		t.Fatal("tenant key scopes collided")
	}
	if _, err := s.Evidence("tenant-b", ids[0]); !errors.Is(err, ErrNotFound) {
		t.Fatal("existing tenant accessed another tenant")
	}
}

func TestConcurrentWrites(t *testing.T) {
	s := openTest(t)
	req, _ := request(1)
	var wg sync.WaitGroup
	var mu sync.Mutex
	createdCount := 0
	ids := map[string]bool{}
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			e, created, err := s.Ingest(req)
			if err != nil {
				t.Error(err)
				return
			}
			mu.Lock()
			defer mu.Unlock()
			if created {
				createdCount++
			}
			ids[e.ID] = true
		}()
	}
	wg.Wait()
	if createdCount != 1 || len(ids) != 1 {
		t.Fatal("concurrent retries duplicated evidence")
	}
	batches := map[string]bool{}
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b, err := s.Freeze(req.Tenant, "batch", 10)
			if err != nil {
				t.Error(err)
				return
			}
			mu.Lock()
			defer mu.Unlock()
			batches[b.ID] = true
		}()
	}
	wg.Wait()
	outbox, err := s.Outbox(req.Tenant, 10)
	if err != nil || len(outbox) != 1 || len(batches) != 1 {
		t.Fatal("concurrent freeze duplicated work")
	}
}

func TestConcurrentDocumentIngestRetries(t *testing.T) {
	s := openTest(t)
	doc := []byte("document upload through HTTP wrapper")
	req := DocumentIngestRequest{
		Tenant:         "tenant-a",
		Key:            "upload-key",
		Source:         Source{"paperless", "object-1", "version-1"},
		Representation: commitment.Original,
		MaxBytes:       1024,
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	createdCount := 0
	ids := map[string]bool{}
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			next := req
			next.Document = bytes.NewReader(doc)
			e, created, err := s.IngestDocument(next)
			if err != nil {
				t.Error(err)
				return
			}
			mu.Lock()
			defer mu.Unlock()
			if created {
				createdCount++
			}
			ids[e.ID] = true
		}()
	}
	wg.Wait()
	if createdCount != 1 || len(ids) != 1 {
		t.Fatal("concurrent document ingestion duplicated evidence")
	}
	req.Document = bytes.NewReader([]byte("changed document"))
	if _, _, err := s.IngestDocument(req); !errors.Is(err, ErrConflict) {
		t.Fatal("changed document retry accepted")
	}
	req.Document = nil
	if _, _, err := s.IngestDocument(req); !errors.Is(err, ErrInvalid) {
		t.Fatal("nil document was not invalid")
	}
}

func TestTransactionsRollBackAllIndexes(t *testing.T) {
	s := openTest(t)
	req, _ := request(1)
	injected := errors.New("rollback")
	if err := s.db.Update(func(tx *bolt.Tx) error {
		if _, _, err := ingest(tx, req); err != nil {
			return err
		}
		return injected
	}); !errors.Is(err, injected) {
		t.Fatal(err)
	}
	e, created, err := s.Ingest(req)
	if err != nil || !created || e.Sequence != 1 {
		t.Fatal("failed ingestion leaked indexes or sequence")
	}
	if err := s.db.Update(func(tx *bolt.Tx) error {
		if _, err := freeze(tx, req.Tenant, "batch", 10); err != nil {
			return err
		}
		return injected
	}); !errors.Is(err, injected) {
		t.Fatal(err)
	}
	e, err = s.Evidence(req.Tenant, e.ID)
	if err != nil || e.BatchID != "" {
		t.Fatal("failed freeze modified evidence")
	}
	work, err := s.Outbox(req.Tenant, 10)
	if err != nil || len(work) != 0 {
		t.Fatal("failed freeze leaked outbox")
	}
	b, err := s.Freeze(req.Tenant, "batch", 10)
	if err != nil || len(b.EvidenceIDs) != 1 {
		t.Fatal("rollback lost pending evidence")
	}
}

func TestProcessRestart(t *testing.T) {
	if path := os.Getenv("SIGMAPROOF_STORE_TEST_CHILD"); path != "" {
		s, err := Open(path)
		if err != nil {
			t.Fatal(err)
		}
		req, _ := request(7)
		if _, _, err := s.Ingest(req); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Freeze(req.Tenant, "batch", 10); err != nil {
			t.Fatal(err)
		}
		os.Exit(0) // Intentionally skip Close and all defers after committed transactions.
	}
	path := filepath.Join(t.TempDir(), "restart.db")
	child := exec.Command(os.Args[0], "-test.run=^TestProcessRestart$")
	child.Env = append(os.Environ(), "SIGMAPROOF_STORE_TEST_CHILD="+path)
	if output, err := child.CombinedOutput(); err != nil {
		t.Fatalf("child failed: %s: %v", output, err)
	}
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	req, doc := request(7)
	e, created, err := s.Ingest(req)
	if err != nil || created || e.BatchID == "" {
		t.Fatal("acknowledged ingestion/freeze lost after process exit")
	}
	b, err := s.Freeze(req.Tenant, "batch", 10)
	if err != nil || b.ID != e.BatchID {
		t.Fatal("freeze retry changed after restart")
	}
	work, err := s.Outbox(req.Tenant, 10)
	if err != nil || len(work) != 1 || !bytes.Equal(work[0].Manifest, b.Manifest) {
		t.Fatal("pending submission lost after restart")
	}
	p, err := s.UnanchoredPackage(req.Tenant, e.ID)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := p.MarshalBinary()
	if report := proof.Verify(bytes.NewReader(doc), bytes.NewReader(data), 1024); !report.IntegrityVerified() || report.Anchor.Status != proof.Unavailable {
		t.Fatal("restart export failed")
	}
}

func TestPermissionsLockSchemaAndLimits(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "private", "store.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("database not private")
	}
	if second, err := Open(path); err == nil {
		second.Close()
		t.Fatal("second writer acquired database")
	}
	s.Close()
	if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	if bad, err := Open(path); err == nil {
		bad.Close()
		t.Fatal("accepted world-readable private database")
	}
	os.Chmod(path, 0600)
	symlink := filepath.Join(dir, "link")
	os.Symlink(path, symlink)
	if bad, err := Open(symlink); err == nil {
		bad.Close()
		t.Fatal("followed database symlink")
	}
	db, err := bolt.Open(path, 0600, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Update(func(tx *bolt.Tx) error { return tx.Bucket([]byte("schema")).Put([]byte("version"), []byte{99}) }); err != nil {
		t.Fatal(err)
	}
	db.Close()
	if bad, err := Open(path); err == nil {
		bad.Close()
		t.Fatal("accepted future schema")
	}
	s = openTest(t)
	req, _ := request(0)
	req.Key = string(bytes.Repeat([]byte{'x'}, 257))
	if _, _, err := s.Ingest(req); !errors.Is(err, ErrInvalid) {
		t.Fatal("oversized input accepted")
	}
	for _, limit := range []int{0, -1, MaxBatchEntries + 1} {
		if _, err := s.Freeze("tenant-a", "batch", limit); !errors.Is(err, ErrInvalid) {
			t.Fatal("invalid batch limit accepted")
		}
	}
}

func TestCorruptBatchAndOutboxFailClosed(t *testing.T) {
	s := openTest(t)
	req, _ := request(1)
	e, _, err := s.Ingest(req)
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.Freeze(req.Tenant, "batch", 10)
	if err != nil {
		t.Fatal(err)
	}
	err = s.db.Update(func(tx *bolt.Tx) error {
		tenant, err := tenant(tx, req.Tenant, false)
		if err != nil {
			return err
		}
		changed := bytes.Clone(b.Manifest)
		changed[29] ^= 1
		return tenant.Bucket([]byte("outbox")).Put([]byte(b.ID), changed)
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Outbox(req.Tenant, 10); !errors.Is(err, ErrCorrupt) {
		t.Fatal("changed outbox payload accepted")
	}
	err = s.db.Update(func(tx *bolt.Tx) error {
		tenant, err := tenant(tx, req.Tenant, false)
		if err != nil {
			return err
		}
		b.Manifest[29] ^= 1
		return put(tenant.Bucket([]byte("batches")), []byte(b.ID), b)
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.UnanchoredPackage(req.Tenant, e.ID); !errors.Is(err, ErrCorrupt) {
		t.Fatal("changed persisted root exported")
	}
}

func TestForeignDatabaseAndClosedStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "foreign.db")
	db, err := bolt.Open(path, 0600, nil)
	if err != nil {
		t.Fatal(err)
	}
	err = db.Update(func(tx *bolt.Tx) error { _, err := tx.CreateBucket([]byte("foreign")); return err })
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
	if s, err := Open(path); err == nil {
		s.Close()
		t.Fatal("foreign database initialized")
	}
	db, err = bolt.Open(path, 0600, nil)
	if err != nil {
		t.Fatal(err)
	}
	err = db.View(func(tx *bolt.Tx) error {
		if tx.Bucket([]byte("schema")) != nil {
			return errors.New("schema added to foreign DB")
		}
		return nil
	})
	db.Close()
	if err != nil {
		t.Fatal(err)
	}
	s := openTest(t)
	s.Close()
	req, _ := request(1)
	e, created, err := s.Ingest(req)
	if err == nil || created || e != (Evidence{}) {
		t.Fatal("closed database returned success")
	}
	b, err := s.Freeze(req.Tenant, "batch", 10)
	if err == nil || b.ID != "" {
		t.Fatal("closed database returned frozen batch")
	}
}

func TestConcurrentDistinctBatchesDoNotOverlap(t *testing.T) {
	s := openTest(t)
	for i := 0; i < 20; i++ {
		req, _ := request(i)
		if _, _, err := s.Ingest(req); err != nil {
			t.Fatal(err)
		}
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	seen := map[string]bool{}
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			b, err := s.Freeze("tenant-a", fmt.Sprintf("batch-%d", n), 5)
			if err != nil {
				t.Error(err)
				return
			}
			mu.Lock()
			defer mu.Unlock()
			for _, id := range b.EvidenceIDs {
				if seen[id] {
					t.Error("evidence assigned to two batches")
				}
				seen[id] = true
			}
		}(i)
	}
	wg.Wait()
	if len(seen) != 20 {
		t.Fatal("lost concurrent batch members")
	}
}
