// Package merkle implements the SHA-256 tree construction in RFC 6962 section 2.1.
// It proves membership relative to a supplied root, not anchor authenticity.
package merkle

import (
	"crypto/sha256"
	"errors"
	"math/bits"
)

// Hash is a SHA-256 digest. Inputs to Root and Path are raw entries, not leaf hashes.
type Hash [sha256.Size]byte

func leaf(data []byte) Hash {
	h := sha256.New()
	h.Write([]byte{0})
	h.Write(data)
	var out Hash
	copy(out[:], h.Sum(nil))
	return out
}

func node(left, right Hash) Hash {
	var data [65]byte
	data[0] = 1
	copy(data[1:33], left[:])
	copy(data[33:], right[:])
	return sha256.Sum256(data[:])
}

func split(size uint64) uint64 { return uint64(1) << (bits.Len64(size-1) - 1) }

// Root returns the root of ordered entries; the empty root is SHA-256(empty).
func Root(entries [][]byte) Hash {
	switch len(entries) {
	case 0:
		return sha256.Sum256(nil)
	case 1:
		return leaf(entries[0])
	default:
		k := int(split(uint64(len(entries))))
		return node(Root(entries[:k]), Root(entries[k:]))
	}
}

// Path returns sibling hashes ordered from the leaf toward the root.
func Path(entries [][]byte, index uint64) ([]Hash, error) {
	if index >= uint64(len(entries)) {
		return nil, errors.New("leaf index outside tree")
	}
	if len(entries) == 1 {
		return []Hash{}, nil
	}
	k := split(uint64(len(entries)))
	if index < k {
		path, err := Path(entries[:int(k)], index)
		return append(path, Root(entries[int(k):])), err
	}
	path, err := Path(entries[int(k):], index-k)
	return append(path, Root(entries[:int(k)])), err
}

// Verify checks exact path length and membership for a nonempty tree.
// The caller must authenticate root and size together. A root alone does not
// authenticate a manifest, publication, time, or claimed tree size.
func Verify(entry []byte, index, size uint64, path []Hash, root Hash) error {
	if size == 0 || index >= size {
		return errors.New("invalid tree size or leaf index")
	}
	if len(path) > 64 {
		return errors.New("path exceeds maximum depth")
	}
	used := 0
	var rebuild func(uint64, uint64) (Hash, error)
	rebuild = func(i, n uint64) (Hash, error) {
		if n == 1 {
			return leaf(entry), nil
		}
		k := split(n)
		var child Hash
		var err error
		if i < k {
			child, err = rebuild(i, k)
		} else {
			child, err = rebuild(i-k, n-k)
		}
		if err != nil {
			return Hash{}, err
		}
		if used >= len(path) {
			return Hash{}, errors.New("truncated inclusion path")
		}
		sibling := path[used]
		used++
		if i < k {
			return node(child, sibling), nil
		}
		return node(sibling, child), nil
	}
	got, err := rebuild(index, size)
	if err != nil {
		return err
	}
	if used != len(path) {
		return errors.New("extra inclusion path nodes")
	}
	if got != root {
		return errors.New("inclusion root mismatch")
	}
	return nil
}
