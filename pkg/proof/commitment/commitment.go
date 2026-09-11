// Package commitment implements the experimental SEP-2 exact-byte commitment
// profile. It makes no statement about document truth, provenance or publication.
package commitment

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"io"
	"math"
)

const (
	// Version identifies the experimental encoding, not a stable protocol release.
	Version        uint8 = 1
	SHA256         uint8 = 1
	MetadataAbsent uint8 = 0
	domain               = "sigmaproof:commitment\x00"
)

// Representation identifies which exact source representation was hashed.
// It is a claim made by the producer, not an authenticated source identity.
type Representation uint8

const (
	Original Representation = 1
	Derived  Representation = 2
)

type Digest [sha256.Size]byte

// Witness is private verification material. Never publish it to an anchor,
// include it in logs, or treat it as a source identifier. Persist it once per
// ingestion: retries must reuse the existing witness rather than create another.
type Witness struct {
	Version        uint8
	Algorithm      uint8
	Representation Representation
	Metadata       uint8
	DocumentDigest Digest
	Nonce          [32]byte
}

// Encode returns the private fixed-width preimage described in draft SEP-2.
// Unknown versions, algorithms, representations and metadata states are rejected.
func Encode(w Witness) ([]byte, error) {
	if w.Version != Version {
		return nil, errors.New("unsupported commitment version")
	}
	if w.Algorithm != SHA256 {
		return nil, errors.New("unsupported digest algorithm")
	}
	if w.Representation != Original && w.Representation != Derived {
		return nil, errors.New("unsupported document representation")
	}
	if w.Metadata != MetadataAbsent {
		return nil, errors.New("metadata commitments are unsupported")
	}
	out := make([]byte, 0, len(domain)+4+64)
	out = append(out, domain...)
	out = append(out, w.Version, w.Algorithm, byte(w.Representation), w.Metadata)
	out = append(out, w.DocumentDigest[:]...)
	out = append(out, w.Nonce[:]...)
	return out, nil
}

// Compute returns the opaque commitment for an existing private witness.
// Call New for new evidence so that the nonce is generated securely.
func Compute(w Witness) (Digest, error) {
	preimage, err := Encode(w)
	if err != nil {
		return Digest{}, err
	}
	return sha256.Sum256(preimage), nil
}

// New streams exact bytes, enforces a caller-selected byte limit, and generates
// a fresh 32-byte CSPRNG nonce. It returns no usable evidence on error.
// The limit is operational policy, not part of the commitment encoding.
func New(document io.Reader, representation Representation, maxBytes int64) (Witness, Digest, error) {
	w := Witness{Version: Version, Algorithm: SHA256, Representation: representation, Metadata: MetadataAbsent}
	if _, err := Encode(w); err != nil {
		return Witness{}, Digest{}, err
	}
	digest, err := hashDocument(document, maxBytes)
	if err != nil {
		return Witness{}, Digest{}, err
	}
	w.DocumentDigest = digest
	if _, err := rand.Read(w.Nonce[:]); err != nil {
		return Witness{}, Digest{}, err
	}
	result, err := Compute(w)
	if err != nil {
		return Witness{}, Digest{}, err
	}
	return w, result, nil
}

// Verify checks exact document bytes against the witness and expected commitment.
// It does not authenticate the expected commitment or anchor; callers must bind
// it to the batch/manifest and verify publication separately.
func Verify(document io.Reader, w Witness, expected Digest, maxBytes int64) error {
	actual, err := Compute(w)
	if err != nil {
		return err
	}
	if subtle.ConstantTimeCompare(actual[:], expected[:]) != 1 {
		return errors.New("commitment mismatch")
	}
	digest, err := hashDocument(document, maxBytes)
	if err != nil {
		return err
	}
	if subtle.ConstantTimeCompare(digest[:], w.DocumentDigest[:]) != 1 {
		return errors.New("document digest mismatch")
	}
	return nil
}

func hashDocument(document io.Reader, maxBytes int64) (Digest, error) {
	if document == nil {
		return Digest{}, errors.New("document reader is required")
	}
	if maxBytes < 0 || maxBytes == math.MaxInt64 {
		return Digest{}, errors.New("invalid document byte limit")
	}
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(document, maxBytes+1))
	if err != nil {
		return Digest{}, err
	}
	if n > maxBytes {
		return Digest{}, errors.New("document exceeds byte limit")
	}
	var result Digest
	copy(result[:], h.Sum(nil))
	return result, nil
}
