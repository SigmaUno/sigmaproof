package proof

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/SigmaUno/sigmaproof/pkg/proof/batch"
	"github.com/SigmaUno/sigmaproof/pkg/proof/commitment"
	"github.com/SigmaUno/sigmaproof/pkg/proof/merkle"
)

func localPackage(t *testing.T) ([]byte, []byte) {
	t.Helper()
	doc := []byte("synthetic document")
	p, err := CreateUnanchored(bytes.NewReader(doc), commitment.Original, 1024)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := p.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	return doc, encoded
}

func TestPythonPackages(t *testing.T) {
	data, err := os.ReadFile("testdata/unanchored-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	var vectors []struct{ Name, Document, Package string }
	if err := json.Unmarshal(data, &vectors); err != nil {
		t.Fatal(err)
	}
	for _, v := range vectors {
		t.Run(v.Name, func(t *testing.T) {
			doc, err := hex.DecodeString(v.Document)
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := hex.DecodeString(v.Package)
			if err != nil {
				t.Fatal(err)
			}
			p, err := Parse(encoded)
			if err != nil {
				t.Fatal(err)
			}
			roundtrip, err := p.MarshalBinary()
			if err != nil || !bytes.Equal(roundtrip, encoded) {
				t.Fatal("fixture did not roundtrip")
			}
			result := Verify(bytes.NewReader(doc), bytes.NewReader(encoded), int64(len(doc)))
			if !result.IntegrityVerified() || result.Anchor.Status != Unavailable {
				t.Fatalf("unexpected report: %+v", result)
			}
			// Returned structures do not alias the private input buffer.
			for i := range encoded {
				encoded[i] ^= 255
			}
			again, _ := p.MarshalBinary()
			if !bytes.Equal(roundtrip, again) {
				t.Fatal("decoder retained mutable input")
			}
		})
	}
}

func TestTamperingSeparatesAssertions(t *testing.T) {
	doc, encoded := localPackage(t)
	cases := []struct {
		name                            string
		offset                          int
		documentStatus, inclusionStatus Status
	}{
		{"nonce", 67, Verified, Failed},
		{"document-digest", 35, Failed, Failed},
		{"root", 128, Verified, Failed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			altered := append([]byte{}, encoded...)
			altered[tc.offset] ^= 1
			report := Verify(bytes.NewReader(doc), bytes.NewReader(altered), 1024)
			if report.Format.Status != Verified || report.Document.Status != tc.documentStatus || report.Inclusion.Status != tc.inclusionStatus || report.Anchor.Status != Unavailable || report.IntegrityVerified() {
				t.Fatalf("wrong report: %+v", report)
			}
		})
	}
	report := Verify(bytes.NewReader([]byte("changed")), bytes.NewReader(encoded), 1024)
	if report.Document.Status != Failed || report.Inclusion.Status != Verified || report.IntegrityVerified() {
		t.Fatalf("wrong document report: %+v", report)
	}
}

func TestBoundedStrictParser(t *testing.T) {
	_, good := localPackage(t)
	bad := [][]byte{nil, good[:169], append(append([]byte{}, good...), 0), append(append([]byte{}, good...), good...), bytes.Repeat([]byte{0}, MaxPackageBytes+1)}
	for _, offset := range []int{0, 9, 30, 99, 115, 168} {
		b := append([]byte{}, good...)
		b[offset] ^= 255
		bad = append(bad, b)
	}
	for _, size := range []uint64{0, batch.MaxLeaves + 1, ^uint64(0)} {
		b := append([]byte{}, good...)
		binary.BigEndian.PutUint64(b[120:128], size)
		bad = append(bad, b)
	}
	b := append([]byte{}, good...)
	binary.BigEndian.PutUint64(b[160:168], 1)
	bad = append(bad, b)
	// Size 2 requires one sibling, even though the framing declares zero siblings.
	b = append([]byte{}, good...)
	binary.BigEndian.PutUint64(b[120:128], 2)
	bad = append(bad, b)
	for _, data := range bad {
		p, err := Parse(data)
		if err == nil || p.Path != nil || p.Witness != (commitment.Witness{}) {
			t.Fatal("malformed package returned usable evidence")
		}
	}
	for _, offset := range []int{8, 31, 32, 33, 34, 116, 117, 118, 119, 169} {
		altered := append([]byte{}, good...)
		altered[offset] = 255
		if _, err := Parse(altered); !errors.Is(err, ErrUnsupported) {
			t.Fatalf("offset %d: %v", offset, err)
		}
		if report := Verify(nil, bytes.NewReader(altered), 0); report.Format.Status != Unsupported || report.Document.Status != Unavailable {
			t.Fatal("unsupported profile treated as validated")
		}
	}
}

type endlessReader struct{ read int }

func (r *endlessReader) Read(p []byte) (int, error) { clear(p); r.read += len(p); return len(p), nil }

type failReader struct{}

func (failReader) Read([]byte) (int, error) { return 0, errors.New("synthetic read error") }

func TestReaderBoundsAndFailures(t *testing.T) {
	r := &endlessReader{}
	if _, err := Read(r); err == nil || r.read != MaxPackageBytes+1 {
		t.Fatalf("package read limit: %d, %v", r.read, err)
	}
	if _, err := Read(nil); err == nil {
		t.Fatal("nil reader accepted")
	}
	if _, err := Read(failReader{}); err == nil {
		t.Fatal("read error ignored")
	}
	doc, pkg := localPackage(t)
	report := Verify(bytes.NewReader(doc), bytes.NewReader(pkg), 1)
	if report.Document.Status != Failed || report.IntegrityVerified() {
		t.Fatal("document limit ignored")
	}
	report = Verify(failReader{}, bytes.NewReader(pkg), 1024)
	if report.Document.Status != Failed {
		t.Fatal("document read error ignored")
	}
}

func FuzzPackage(f *testing.F) {
	p, _ := CreateUnanchored(bytes.NewReader([]byte("seed")), commitment.Original, 1024)
	seed, _ := p.MarshalBinary()
	f.Add(seed)
	f.Add([]byte{})
	f.Add(bytes.Repeat([]byte{255}, MaxPackageBytes))
	f.Fuzz(func(t *testing.T, data []byte) {
		p, err := Parse(data)
		if err != nil {
			return
		}
		encoded, err := p.MarshalBinary()
		if err != nil || !bytes.Equal(data, encoded) {
			t.Fatal("noncanonical package accepted")
		}
		report := Verify(bytes.NewReader(nil), bytes.NewReader(data), 0)
		if report.Anchor.Status != Unavailable {
			t.Fatal("unanchored package authenticated")
		}
	})
}

func TestMaximumPathFraming(t *testing.T) {
	_, encoded := localPackage(t)
	p, err := Parse(encoded)
	if err != nil {
		t.Fatal(err)
	}
	p.Manifest.Size = batch.MaxLeaves
	p.Path = make([]merkle.Hash, 16)
	encoded, err = p.MarshalBinary()
	if err != nil || len(encoded) != MaxPackageBytes {
		t.Fatalf("maximum framing: %d, %v", len(encoded), err)
	}
	if _, err := Parse(encoded); err != nil {
		t.Fatal(err)
	}
	p.Path = append(p.Path, merkle.Hash{})
	if _, err := p.MarshalBinary(); err == nil {
		t.Fatal("oversized path encoded")
	}
}
