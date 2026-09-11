package merkle

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

func TestPythonVectors(t *testing.T) {
	data, err := os.ReadFile("testdata/rfc6962.json")
	if err != nil {
		t.Fatal(err)
	}
	var vectors []struct {
		Entries []string
		Index   uint64
		Root    string
		Path    []string
	}
	if err := json.Unmarshal(data, &vectors); err != nil {
		t.Fatal(err)
	}
	for _, v := range vectors {
		entries := make([][]byte, len(v.Entries))
		for i, s := range v.Entries {
			entries[i], err = hex.DecodeString(s)
			if err != nil {
				t.Fatal(err)
			}
		}
		root := Root(entries)
		if hex.EncodeToString(root[:]) != v.Root {
			t.Fatalf("root mismatch size=%d", len(entries))
		}
		if len(entries) == 0 {
			if Verify(nil, 0, 0, nil, root) == nil {
				t.Fatal("accepted empty tree proof")
			}
			continue
		}
		path, err := Path(entries, v.Index)
		if err != nil {
			t.Fatal(err)
		}
		if len(path) != len(v.Path) {
			t.Fatal("path length mismatch")
		}
		for i, h := range path {
			if hex.EncodeToString(h[:]) != v.Path[i] {
				t.Fatal("path differs from Python fixture")
			}
		}
		entry := entries[v.Index]
		if err := Verify(entry, v.Index, uint64(len(entries)), path, root); err != nil {
			t.Fatal(err)
		}
		if Verify(append(append([]byte{}, entry...), 255), v.Index, uint64(len(entries)), path, root) == nil {
			t.Fatal("accepted altered leaf")
		}
		if Verify(entry, v.Index, uint64(len(entries)), append(append([]Hash{}, path...), Hash{}), root) == nil {
			t.Fatal("accepted extra sibling")
		}
		if len(path) > 0 {
			if Verify(entry, v.Index, uint64(len(entries)), path[:len(path)-1], root) == nil {
				t.Fatal("accepted truncated path")
			}
			altered := append([]Hash{}, path...)
			altered[0][0] ^= 1
			if Verify(entry, v.Index, uint64(len(entries)), altered, root) == nil {
				t.Fatal("accepted altered sibling")
			}
		}
		if len(entries) > 1 && Verify(entry, (v.Index+1)%uint64(len(entries)), uint64(len(entries)), path, root) == nil {
			t.Fatal("accepted wrong index")
		}
	}
}

func TestInvalidBounds(t *testing.T) {
	if _, err := Path(nil, 0); err == nil {
		t.Fatal("empty tree accepted")
	}
	if Verify(nil, 1, 1, nil, Hash{}) == nil {
		t.Fatal("out of range index accepted")
	}
	if Verify(nil, 0, ^uint64(0), make([]Hash, 65), Hash{}) == nil {
		t.Fatal("oversized path accepted")
	}
	entries := [][]byte{[]byte("a"), []byte("b"), []byte("c")}
	path, _ := Path(entries, 0)
	if Verify(entries[0], 0, 2, path, Root(entries)) == nil {
		t.Fatal("wrong size accepted")
	}
}

func FuzzVerify(f *testing.F) {
	f.Add([]byte("entry"), uint64(0), uint64(1), []byte{})
	f.Add([]byte{}, ^uint64(0)-1, ^uint64(0), make([]byte, 64*32))
	f.Fuzz(func(t *testing.T, entry []byte, index, size uint64, raw []byte) {
		if len(raw) > 65*32 {
			return
		}
		path := make([]Hash, len(raw)/32)
		for i := range path {
			copy(path[i][:], raw[i*32:(i+1)*32])
		}
		_ = Verify(entry, index, size, path, Hash{})
	})
}
