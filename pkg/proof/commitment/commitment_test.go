package commitment

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math"
	"os"
	"testing"

	"github.com/SigmaUno/sigmaproof/pkg/proof/merkle"
)

func decode(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestPythonVectors(t *testing.T) {
	data, err := os.ReadFile("testdata/sep2-draft.json")
	if err != nil {
		t.Fatal(err)
	}
	var vectors []struct {
		Name           string
		Document       string
		Representation Representation
		Nonce          string
		DocumentDigest string `json:"document_digest"`
		Preimage       string
		Commitment     string
	}
	if err := json.Unmarshal(data, &vectors); err != nil {
		t.Fatal(err)
	}
	for _, v := range vectors {
		t.Run(v.Name, func(t *testing.T) {
			w := Witness{Version: Version, Algorithm: SHA256, Representation: v.Representation}
			copy(w.Nonce[:], decode(t, v.Nonce))
			copy(w.DocumentDigest[:], decode(t, v.DocumentDigest))
			preimage, err := Encode(w)
			if err != nil || !bytes.Equal(preimage, decode(t, v.Preimage)) {
				t.Fatalf("preimage mismatch: %v", err)
			}
			got, err := Compute(w)
			if err != nil || hex.EncodeToString(got[:]) != v.Commitment {
				t.Fatalf("commitment mismatch: %v", err)
			}
			doc := decode(t, v.Document)
			if err := Verify(bytes.NewReader(doc), w, got, int64(len(doc))); err != nil {
				t.Fatal(err)
			}
			for _, mutate := range []func(*Witness){
				func(x *Witness) { x.Nonce[0] ^= 1 },
				func(x *Witness) { x.DocumentDigest[0] ^= 1 },
				func(x *Witness) {
					if x.Representation == Original {
						x.Representation = Derived
					} else {
						x.Representation = Original
					}
				},
				func(x *Witness) { x.Algorithm = 2 },
				func(x *Witness) { x.Version = 2 },
				func(x *Witness) { x.Metadata = 1 },
			} {
				changed := w
				mutate(&changed)
				if Verify(bytes.NewReader(doc), changed, got, int64(len(doc))) == nil {
					t.Fatal("accepted altered witness")
				}
			}
			changedDoc := append(append([]byte{}, doc...), 1)
			if Verify(bytes.NewReader(changedDoc), w, got, int64(len(changedDoc))) == nil {
				t.Fatal("accepted changed document")
			}
		})
	}
}

func TestFreshNoncesAndBatchMembership(t *testing.T) {
	doc := []byte("synthetic document")
	w, first, err := New(bytes.NewReader(doc), Original, 1024)
	if err != nil {
		t.Fatal(err)
	}
	other, second, err := New(bytes.NewReader(doc), Original, 1024)
	if err != nil {
		t.Fatal(err)
	}
	if w.Nonce == other.Nonce || first == second {
		t.Fatal("fresh evidence reused randomness")
	}
	if err := Verify(bytes.NewReader(doc), w, first, 1024); err != nil {
		t.Fatal(err)
	}
	entries := [][]byte{first[:], second[:]}
	path, err := merkle.Path(entries, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := merkle.Verify(first[:], 0, 2, path, merkle.Root(entries)); err != nil {
		t.Fatal(err)
	}
	if merkle.Verify(w.DocumentDigest[:], 0, 2, path, merkle.Root(entries)) == nil {
		t.Fatal("document digest accepted in place of randomized commitment")
	}
}

var readFailure = errors.New("synthetic read failure")

type brokenReader struct{}

func (brokenReader) Read(p []byte) (int, error) { copy(p, "x"); return 1, readFailure }

type countingReader struct{ n int }

func (r *countingReader) Read(p []byte) (int, error) { r.n += len(p); clear(p); return len(p), nil }

func TestResourceLimitsAndErrors(t *testing.T) {
	for _, tc := range []struct {
		name   string
		reader io.Reader
		limit  int64
	}{
		{"oversized", bytes.NewReader([]byte("ab")), 1},
		{"zero-limit-nonempty", bytes.NewReader([]byte("a")), 0},
		{"negative-limit", bytes.NewReader(nil), -1},
		{"overflow-limit", bytes.NewReader(nil), math.MaxInt64},
		{"nil-reader", nil, 1},
		{"reader-failure", brokenReader{}, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w, c, err := New(tc.reader, Original, tc.limit)
			if err == nil || w != (Witness{}) || c != (Digest{}) {
				t.Fatal("failure returned usable evidence")
			}
		})
	}
	if _, _, err := New(bytes.NewReader(nil), Original, 0); err != nil {
		t.Fatal(err)
	}
	r := &countingReader{}
	if _, _, err := New(r, Original, 8); err == nil || r.n != 9 {
		t.Fatalf("limit read = %d, err = %v", r.n, err)
	}
	if _, _, err := New(bytes.NewReader(nil), Representation(0), 0); err == nil {
		t.Fatal("accepted unspecified representation")
	}
	if _, _, err := New(brokenReader{}, Original, 1); !errors.Is(err, readFailure) {
		t.Fatal("lost read error")
	}
}

func TestStrictPreimageDecode(t *testing.T) {
	w, _, err := New(bytes.NewReader(nil), Original, 0)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := Encode(w)
	parsed, err := Parse(encoded)
	if err != nil || parsed != w {
		t.Fatal("witness roundtrip failed")
	}
	for _, bad := range [][]byte{nil, encoded[:89], append(append([]byte{}, encoded...), 0)} {
		if _, err := Parse(bad); err == nil {
			t.Fatal("invalid framing accepted")
		}
	}
	for _, offset := range []int{0, 21, 22, 23, 24, 25} {
		bad := append([]byte{}, encoded...)
		bad[offset] = 255
		if _, err := Parse(bad); err == nil {
			t.Fatal("invalid preimage accepted")
		}
	}
}
