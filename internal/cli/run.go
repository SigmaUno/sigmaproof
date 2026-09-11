// Package cli implements local proof commands. It never contacts a remote service.
package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"

	"github.com/SigmaUno/sigmaproof/internal/buildinfo"
	"github.com/SigmaUno/sigmaproof/pkg/proof"
	"github.com/SigmaUno/sigmaproof/pkg/proof/commitment"
)

const defaultDocumentLimit = 64 << 20

// Run returns 0 for satisfied requested checks, 1 for invalid evidence/I/O errors,
// 2 for usage errors, and 3 for unsupported or unavailable requested verification.
func Run(args []string, out, errOut io.Writer) int {
	if len(args) == 0 || (len(args) == 1 && (args[0] == "help" || args[0] == "--help" || args[0] == "-h")) {
		fmt.Fprintln(out, "Usage:\n  sigmaproof version\n  sigmaproof create --unanchored --representation original|derived [--max-document-bytes N] DOCUMENT PACKAGE\n  sigmaproof verify [--offline] [--json] [--max-document-bytes N] DOCUMENT PACKAGE\nExperimental format. Offline success establishes local integrity only; anchors are unavailable.")
		return 0
	}
	if len(args) == 1 && args[0] == "version" {
		fmt.Fprintln(out, buildinfo.Version)
		return 0
	}
	switch args[0] {
	case "verify":
		return verify(args[1:], out, errOut)
	case "create":
		return create(args[1:], out, errOut)
	default:
		fmt.Fprintln(errOut, "invalid command; use sigmaproof help")
		return 2
	}
}

func verify(args []string, out, errOut io.Writer) int {
	flags := flag.NewFlagSet("verify", flag.ContinueOnError)
	flags.SetOutput(errOut)
	offline := flags.Bool("offline", false, "require local integrity only; does not authenticate an anchor")
	asJSON := flags.Bool("json", false, "write per-check results as JSON")
	limit := flags.Int64("max-document-bytes", defaultDocumentLimit, "maximum document size")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() != 2 || *limit < 0 || *limit == math.MaxInt64 {
		fmt.Fprintln(errOut, "verify requires DOCUMENT PACKAGE and a valid byte limit; flags precede paths")
		return 2
	}
	document, err := openRegular(flags.Arg(0))
	if err != nil {
		fmt.Fprintln(errOut, "cannot open document:", err)
		return 1
	}
	defer document.Close()
	pkg, err := openRegular(flags.Arg(1))
	if err != nil {
		fmt.Fprintln(errOut, "cannot open package:", err)
		return 1
	}
	defer pkg.Close()
	result := proof.Verify(document, pkg, *limit)
	if *asJSON {
		if err := json.NewEncoder(out).Encode(result); err != nil {
			fmt.Fprintln(errOut, "cannot write result:", err)
			return 1
		}
	} else {
		for _, check := range []struct {
			name  string
			value proof.Check
		}{
			{"format", result.Format}, {"document", result.Document}, {"batch inclusion", result.Inclusion}, {"anchor", result.Anchor},
		} {
			if _, err := fmt.Fprintf(out, "%s: %s — %s\n", check.name, check.value.Status, check.value.Reason); err != nil {
				return 1
			}
		}
	}
	if result.Format.Status == proof.Unsupported {
		return 3
	}
	if !result.IntegrityVerified() {
		return 1
	}
	if !*offline {
		return 3
	}
	return 0
}

func create(args []string, out, errOut io.Writer) int {
	flags := flag.NewFlagSet("create", flag.ContinueOnError)
	flags.SetOutput(errOut)
	unanchored := flags.Bool("unanchored", false, "explicitly create experimental local evidence without publication")
	representation := flags.String("representation", "", "original or derived; a producer assertion")
	limit := flags.Int64("max-document-bytes", defaultDocumentLimit, "maximum document size")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() != 2 || !*unanchored || *limit < 0 || *limit == math.MaxInt64 {
		fmt.Fprintln(errOut, "create requires --unanchored, --representation, DOCUMENT PACKAGE and a valid byte limit")
		return 2
	}
	var rep commitment.Representation
	switch *representation {
	case "original":
		rep = commitment.Original
	case "derived":
		rep = commitment.Derived
	default:
		fmt.Fprintln(errOut, "representation must be original or derived")
		return 2
	}
	document, err := openRegular(flags.Arg(0))
	if err != nil {
		fmt.Fprintln(errOut, "cannot open document:", err)
		return 1
	}
	defer document.Close()
	p, err := proof.CreateUnanchored(document, rep, *limit)
	if err != nil {
		fmt.Fprintln(errOut, "cannot create package:", err)
		return 1
	}
	data, err := p.MarshalBinary()
	if err != nil {
		fmt.Fprintln(errOut, "cannot encode package:", err)
		return 1
	}
	if err := writePrivateFile(flags.Arg(1), data); err != nil {
		fmt.Fprintln(errOut, "cannot save package:", err)
		return 1
	}
	fmt.Fprintln(out, "Created experimental unanchored package. No independent publication, timestamp or provenance has been established.")
	return 0
}

func openRegular(path string) (*os.File, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("input must be a regular file")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	info, err = f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		f.Close()
		return nil, fmt.Errorf("input must be a readable regular file")
	}
	return f, nil
}

// writePrivateFile creates the completed private package without overwriting an
// existing destination (including a symlink). A same-directory hard link exposes
// only the complete file. The temporary file is mode 0600 and removed on exit.
func writePrivateFile(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".sigmaproof-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if _, err := f.Write(data); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Link(f.Name(), path)
}
