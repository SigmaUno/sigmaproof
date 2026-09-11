// Package batch implements the experimental SEP-5 public manifest profile.
// Membership verification is local consistency checking, not anchor authentication.
package batch

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"

	"github.com/SigmaUno/sigmaproof/pkg/proof/commitment"
	"github.com/SigmaUno/sigmaproof/pkg/proof/merkle"
)

const (
	Version       uint8 = 1
	RFC6962SHA256 uint8 = 1
	// MaxLeaves bounds this experimental profile to 65536 entries and depth 16.
	MaxLeaves   uint64 = 1 << 16
	EncodedSize        = 61
	domain             = "sigmaproof:batch\x00"
)

// Manifest contains public protocol fields and an opaque root only.
// Hashes of source documents, nonces and source identifiers must never be added.
type Manifest struct {
	Version             uint8
	CommitmentVersion   uint8
	CommitmentAlgorithm uint8
	TreeAlgorithm       uint8
	Size                uint64
	Root                merkle.Hash
}

func (m Manifest) validate() error {
	if m.Version != Version {
		return errors.New("unsupported batch manifest version")
	}
	if m.CommitmentVersion != commitment.Version {
		return errors.New("unsupported commitment profile")
	}
	if m.CommitmentAlgorithm != commitment.SHA256 {
		return errors.New("unsupported commitment algorithm")
	}
	if m.TreeAlgorithm != RFC6962SHA256 {
		return errors.New("unsupported tree algorithm")
	}
	if m.Size == 0 || m.Size > MaxLeaves {
		return errors.New("batch size outside profile limits")
	}
	return nil
}

// New constructs a manifest from ordered opaque commitments. It does not retain
// the input slice. Callers must freeze and persist their ordered batch atomically.
func New(commitments []commitment.Digest) (Manifest, error) {
	m := Manifest{Version: Version, CommitmentVersion: commitment.Version, CommitmentAlgorithm: commitment.SHA256, TreeAlgorithm: RFC6962SHA256, Size: uint64(len(commitments))}
	if err := m.validate(); err != nil {
		return Manifest{}, err
	}
	entries := make([][]byte, len(commitments))
	for i := range commitments {
		entries[i] = commitments[i][:]
	}
	m.Root = merkle.Root(entries)
	return m, nil
}

// MarshalBinary returns the exact 61 public bytes that the provider must bind.
// Anchoring only Root or substituting a manifest digest is not this profile.
func (m Manifest) MarshalBinary() ([]byte, error) {
	if err := m.validate(); err != nil {
		return nil, err
	}
	out := make([]byte, EncodedSize)
	copy(out, domain)
	out[17], out[18], out[19], out[20] = m.Version, m.CommitmentVersion, m.CommitmentAlgorithm, m.TreeAlgorithm
	binary.BigEndian.PutUint64(out[21:29], m.Size)
	copy(out[29:], m.Root[:])
	return out, nil
}

// Parse accepts one exact manifest, rejecting truncation, trailing bytes and
// unsupported fields. It never allocates based on the untrusted tree size.
func Parse(data []byte) (Manifest, error) {
	if len(data) != EncodedSize {
		return Manifest{}, errors.New("invalid manifest length")
	}
	if !bytes.Equal(data[:17], []byte(domain)) {
		return Manifest{}, errors.New("invalid manifest domain")
	}
	m := Manifest{Version: data[17], CommitmentVersion: data[18], CommitmentAlgorithm: data[19], TreeAlgorithm: data[20], Size: binary.BigEndian.Uint64(data[21:29])}
	copy(m.Root[:], data[29:])
	if err := m.validate(); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

// Digest is a local content address for the exact encoded manifest. It is not a
// publication receipt and must not replace the public manifest in the adapter.
func (m Manifest) Digest() ([32]byte, error) {
	data, err := m.MarshalBinary()
	if err != nil {
		return [32]byte{}, err
	}
	return sha256.Sum256(data), nil
}

// Verify checks membership relative to this manifest. The caller must first
// authenticate ALL encoded manifest bytes through independent anchor verification.
func (m Manifest) Verify(c commitment.Digest, index uint64, path []merkle.Hash) error {
	if err := m.validate(); err != nil {
		return err
	}
	return merkle.Verify(c[:], index, m.Size, path, m.Root)
}
