// Package cli implements the command shell for the independent verifier.
package cli

import (
	"fmt"
	"io"

	"github.com/SigmaUno/sigmaproof/internal/buildinfo"
)

// Run returns 0 for help/version, 2 for invalid usage, and 3 for unsupported work.
// Verification must never report success until the real verifier is implemented.
func Run(args []string, out, errOut io.Writer) int {
	if len(args) == 0 || (len(args) == 1 && (args[0] == "help" || args[0] == "--help" || args[0] == "-h")) {
		fmt.Fprintln(out, "Usage: sigmaproof version | verify [--offline] DOCUMENT PACKAGE\nDevelopment skeleton: verification is not implemented.")
		return 0
	}
	if len(args) == 1 && args[0] == "version" {
		fmt.Fprintln(out, buildinfo.Version)
		return 0
	}
	if args[0] == "verify" {
		fmt.Fprintln(errOut, "unsupported: proof verification is not implemented; no evidence was validated")
		return 3
	}
	fmt.Fprintln(errOut, "invalid command; use sigmaproof help")
	return 2
}
