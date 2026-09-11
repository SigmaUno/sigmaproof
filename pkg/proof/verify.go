package proof

import (
	"errors"
	"io"

	"github.com/SigmaUno/sigmaproof/pkg/proof/commitment"
)

type Status string

const (
	Verified    Status = "verified"
	Failed      Status = "failed"
	Unavailable Status = "unavailable"
	Unsupported Status = "unsupported"
)

type Check struct {
	Status Status `json:"status"`
	Reason string `json:"reason"`
}

// Report intentionally has no overall "verified" boolean. Local consistency
// cannot establish independent publication, time, provenance or policy compliance.
type Report struct {
	Format    Check `json:"format"`
	Document  Check `json:"document"`
	Inclusion Check `json:"batch_inclusion"`
	Anchor    Check `json:"anchor"`
}

func initialReport() Report {
	return Report{
		Format:    Check{Unavailable, "package has not been decoded"},
		Document:  Check{Unavailable, "document digest has not been checked"},
		Inclusion: Check{Unavailable, "batch membership has not been checked"},
		Anchor:    Check{Unavailable, "anchor authentication was not performed; no anchor verifier is implemented"},
	}
}

// IntegrityVerified only describes document-to-batch consistency.
func (r Report) IntegrityVerified() bool {
	return r.Format.Status == Verified && r.Document.Status == Verified && r.Inclusion.Status == Verified
}

// Verify reads a bounded package and streams document bytes with no networking.
// It does not evaluate policies or authenticate anchors in this profile.
func Verify(document, encodedPackage io.Reader, maxDocumentBytes int64) Report {
	result := initialReport()
	p, err := Read(encodedPackage)
	if err != nil {
		status := Failed
		if errors.Is(err, ErrUnsupported) {
			status = Unsupported
		}
		result.Format = Check{status, err.Error()}
		return result
	}
	result.Format = Check{Verified, "experimental unanchored package structure is valid"}
	result.Anchor = Check{Unavailable, "unanchored package contains no independently authenticated publication evidence"}
	c, err := commitment.Compute(p.Witness)
	if err != nil {
		result.Inclusion = Check{Failed, err.Error()}
		return result
	}
	if err := p.Manifest.Verify(c, p.Index, p.Path); err != nil {
		result.Inclusion = Check{Failed, "document commitment does not match the carried batch proof"}
	} else {
		result.Inclusion = Check{Verified, "commitment belongs to the carried manifest; manifest is not externally authenticated"}
	}
	if err := commitment.Verify(document, p.Witness, c, maxDocumentBytes); err != nil {
		result.Document = Check{Failed, err.Error()}
	} else {
		result.Document = Check{Verified, "exact document bytes match the private witness"}
	}
	return result
}
