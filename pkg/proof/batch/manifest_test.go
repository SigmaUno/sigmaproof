package batch

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"

	"github.com/SigmaUno/sigmaproof/pkg/proof/commitment"
	"github.com/SigmaUno/sigmaproof/pkg/proof/merkle"
)

func TestPythonVectors(t *testing.T) {
	data, err := os.ReadFile("testdata/sep5-draft.json")
	if err != nil {
		t.Fatal(err)
	}
	var vectors []struct {
		Name                  string
		Commitments           []string
		Root, Encoded, Digest string
	}
	if err := json.Unmarshal(data, &vectors); err != nil {
		t.Fatal(err)
	}
	for _, v := range vectors {
		t.Run(v.Name, func(t *testing.T) {
			cs := make([]commitment.Digest, len(v.Commitments))
			entries := make([][]byte, len(cs))
			for i, text := range v.Commitments {
				raw, err := hex.DecodeString(text)
				if err != nil || len(raw) != 32 {
					t.Fatal("invalid fixture commitment")
				}
				copy(cs[i][:], raw)
				entries[i] = cs[i][:]
			}
			m, err := New(cs)
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := m.MarshalBinary()
			if err != nil || hex.EncodeToString(encoded) != v.Encoded {
				t.Fatalf("encoding mismatch: %v", err)
			}
			if hex.EncodeToString(m.Root[:]) != v.Root {
				t.Fatal("root mismatch")
			}
			digest, err := m.Digest()
			if err != nil || hex.EncodeToString(digest[:]) != v.Digest {
				t.Fatal("digest mismatch")
			}
			decoded, err := Parse(encoded)
			if err != nil || decoded != m {
				t.Fatalf("parse mismatch: %v", err)
			}
			for i, c := range cs {
				path, err := merkle.Path(entries, uint64(i))
				if err != nil {
					t.Fatal(err)
				}
				if err := decoded.Verify(c, uint64(i), path); err != nil {
					t.Fatal(err)
				}
				wrong := decoded
				wrong.Root[0] ^= 1
				if wrong.Verify(c, uint64(i), path) == nil {
					t.Fatal("accepted altered root")
				}
			}
		})
	}
}

func TestStrictParsing(t *testing.T) {
	m, err := New([]commitment.Digest{{1}})
	if err != nil {
		t.Fatal(err)
	}
	good, _ := m.MarshalBinary()
	bad := [][]byte{nil, good[:60], append(append([]byte{}, good...), 0), bytes.Repeat([]byte{0}, 61)}
	for _, offset := range []int{0, 16, 17, 18, 19, 20} {
		b := append([]byte{}, good...)
		b[offset] ^= 0xff
		bad = append(bad, b)
	}
	for _, size := range []uint64{0, MaxLeaves + 1, ^uint64(0)} {
		b := append([]byte{}, good...)
		binary.BigEndian.PutUint64(b[21:29], size)
		bad = append(bad, b)
	}
	for _, b := range bad {
		got, err := Parse(b)
		if err == nil || got != (Manifest{}) {
			t.Fatal("malformed manifest returned usable value")
		}
	}
	// A parser validates structure, not authenticity: altered root bytes are valid
	// syntax, and only a separately authenticated manifest can reject substitution.
	good[29] ^= 1
	changed, err := Parse(good)
	if err != nil || changed.Root == m.Root {
		t.Fatal("unexpected parser behavior")
	}
	if _, err := New(nil); err == nil {
		t.Fatal("empty batch accepted")
	}
	if _, err := New(make([]commitment.Digest, MaxLeaves+1)); err == nil {
		t.Fatal("oversized batch accepted")
	}
}

func TestSizeMustBeAnchoredEvenWhenPathMatches(t *testing.T) {
	cs := []commitment.Digest{{1}, {2}, {3}}
	m, _ := New(cs)
	entries := [][]byte{cs[0][:], cs[1][:], cs[2][:]}
	path, _ := merkle.Path(entries, 0)
	changed := m
	changed.Size = 4
	// A path for index 0 can have the same shape at sizes 3 and 4. Do not
	// mistake membership verification for authentication of the stated tree size.
	if err := changed.Verify(cs[0], 0, path); err != nil {
		t.Fatal(err)
	}
	originalBytes, _ := m.MarshalBinary()
	changedBytes, _ := changed.MarshalBinary()
	originalHash, _ := m.Digest()
	changedHash, _ := changed.Digest()
	if bytes.Equal(originalBytes, changedBytes) || originalHash == changedHash {
		t.Fatal("size not bound by encoding")
	}
}

func TestPrivateWitnessExcluded(t *testing.T) {
	doc := []byte("synthetic private document")
	w, c, err := commitment.New(bytes.NewReader(doc), commitment.Original, 1024)
	if err != nil {
		t.Fatal(err)
	}
	m, err := New([]commitment.Digest{c})
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := m.MarshalBinary()
	if len(encoded) != EncodedSize {
		t.Fatal("unexpected public fields")
	}
	for _, private := range [][]byte{doc, w.Nonce[:], w.DocumentDigest[:]} {
		if bytes.Contains(encoded, private) {
			t.Fatal("private material found in manifest")
		}
	}
	// Changing the caller's batch after construction cannot change the manifest.
	cs := []commitment.Digest{c}
	frozen, _ := New(cs)
	cs[0][0] ^= 1
	if frozen != m {
		t.Fatal("manifest retained mutable input")
	}
}

func TestMaximumBatch(t *testing.T) {
	cs := make([]commitment.Digest, MaxLeaves)
	for i := range cs {
		binary.BigEndian.PutUint64(cs[i][:8], uint64(i))
	}
	m, err := New(cs)
	if err != nil {
		t.Fatal(err)
	}
	entries := make([][]byte, len(cs))
	for i := range cs {
		entries[i] = cs[i][:]
	}
	path, err := merkle.Path(entries, MaxLeaves-1)
	if err != nil || len(path) != 16 {
		t.Fatalf("maximum path: %d, %v", len(path), err)
	}
	if err := m.Verify(cs[len(cs)-1], MaxLeaves-1, path); err != nil {
		t.Fatal(err)
	}
}

func FuzzParse(f *testing.F) {
	m, _ := New([]commitment.Digest{{1}})
	good, _ := m.MarshalBinary()
	f.Add(good)
	f.Add([]byte{})
	f.Add(bytes.Repeat([]byte{255}, EncodedSize))
	f.Fuzz(func(t *testing.T, data []byte) {
		m, err := Parse(data)
		if err != nil {
			return
		}
		encoded, err := m.MarshalBinary()
		if err != nil || !bytes.Equal(encoded, data) {
			t.Fatal("noncanonical accepted manifest")
		}
	})
}
