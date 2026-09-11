package proof

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"math/bits"

	"github.com/SigmaUno/sigmaproof/pkg/proof/batch"
	"github.com/SigmaUno/sigmaproof/pkg/proof/commitment"
	"github.com/SigmaUno/sigmaproof/pkg/proof/merkle"
)

const (
	PackageVersion  = 1
	MaxPackageBytes = 170 + 16*32
	magic           = "SIGMAPRF"
)

var ErrUnsupported = errors.New("unsupported proof package profile or feature")

// Package is the experimental, explicitly unanchored portable evidence profile.
// It contains private material; never submit these bytes to an anchor or log them.
// There are no attachment paths, URLs, timestamps or provider claims in this profile.
type Package struct {
	Witness  commitment.Witness
	Manifest batch.Manifest
	Index    uint64
	Path     []merkle.Hash
}

// pathLength derives the exact RFC 6962 sibling count from index and size.
func pathLength(index, size uint64) int {
	depth := 0
	for size > 1 {
		k := uint64(1) << (bits.Len64(size-1) - 1)
		if index < k {
			size = k
		} else {
			index -= k
			size -= k
		}
		depth++
	}
	return depth
}

// MarshalBinary validates structure, not document integrity or anchor authenticity.
func (p Package) MarshalBinary() ([]byte, error) {
	witness, err := commitment.Encode(p.Witness)
	if err != nil {
		return nil, errors.Join(ErrUnsupported, err)
	}
	manifest, err := p.Manifest.MarshalBinary()
	if err != nil {
		return nil, err
	}
	if p.Index >= p.Manifest.Size {
		return nil, errors.New("leaf index outside manifest")
	}
	if len(p.Path) != pathLength(p.Index, p.Manifest.Size) {
		return nil, errors.New("incorrect inclusion path length")
	}
	out := make([]byte, 170+len(p.Path)*32)
	copy(out, magic)
	out[8] = PackageVersion
	copy(out[9:99], witness)
	copy(out[99:160], manifest)
	binary.BigEndian.PutUint64(out[160:168], p.Index)
	out[168] = byte(len(p.Path))
	for i := range p.Path {
		copy(out[169+i*32:169+(i+1)*32], p.Path[i][:])
	}
	// Last byte is anchor count, zero for this unanchored development profile.
	return out, nil
}

// Parse accepts exactly one bounded binary package, with no optional extensions.
// Unknown versions/algorithms and any nonzero anchor count are unsupported.
func Parse(data []byte) (Package, error) {
	if len(data) < 170 || len(data) > MaxPackageBytes {
		return Package{}, errors.New("invalid package length")
	}
	if !bytes.Equal(data[:8], []byte(magic)) {
		return Package{}, errors.New("invalid package magic")
	}
	if data[8] != PackageVersion {
		return Package{}, ErrUnsupported
	}
	count := int(data[168])
	if count > 16 || len(data) != 170+32*count {
		return Package{}, errors.New("invalid package path framing")
	}
	if data[len(data)-1] != 0 {
		return Package{}, ErrUnsupported
	}
	// Classify unsupported semantics before calling the strict primitive parsers.
	if data[31] != 1 || data[32] != 1 || (data[33] != 1 && data[33] != 2) || data[34] != 0 ||
		data[116] != 1 || data[117] != 1 || data[118] != 1 || data[119] != 1 {
		return Package{}, ErrUnsupported
	}
	witness, err := commitment.Parse(data[9:99])
	if err != nil {
		return Package{}, err
	}
	manifest, err := batch.Parse(data[99:160])
	if err != nil {
		return Package{}, err
	}
	index := binary.BigEndian.Uint64(data[160:168])
	if index >= manifest.Size {
		return Package{}, errors.New("leaf index outside manifest")
	}
	if count != pathLength(index, manifest.Size) {
		return Package{}, errors.New("incorrect inclusion path length")
	}
	p := Package{Witness: witness, Manifest: manifest, Index: index, Path: make([]merkle.Hash, count)}
	for i := range p.Path {
		copy(p.Path[i][:], data[169+i*32:169+(i+1)*32])
	}
	return p, nil
}

// Read consumes at most MaxPackageBytes+1 bytes and does not follow any references.
// The caller owns reader cancellation/deadlines.
func Read(r io.Reader) (Package, error) {
	if r == nil {
		return Package{}, errors.New("package reader is required")
	}
	data, err := io.ReadAll(io.LimitReader(r, MaxPackageBytes+1))
	if err != nil {
		return Package{}, err
	}
	return Parse(data)
}

// CreateUnanchored creates a single-leaf local package. It does not submit data
// or claim an external timestamp. It is intended for development and local tests.
func CreateUnanchored(document io.Reader, representation commitment.Representation, maxBytes int64) (Package, error) {
	w, c, err := commitment.New(document, representation, maxBytes)
	if err != nil {
		return Package{}, err
	}
	m, err := batch.New([]commitment.Digest{c})
	if err != nil {
		return Package{}, err
	}
	return Package{Witness: w, Manifest: m, Path: []merkle.Hash{}}, nil
}
