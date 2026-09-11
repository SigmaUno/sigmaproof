package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestVerificationNeverSucceeds(t *testing.T) {
	for _, args := range [][]string{{"verify"}, {"verify", "--offline", "missing.pdf", "missing.sigmaproof"}} {
		var out, errOut bytes.Buffer
		if code := Run(args, &out, &errOut); code != 3 {
			t.Fatalf("exit = %d", code)
		}
		if out.Len() != 0 || !strings.Contains(errOut.String(), "no evidence was validated") {
			t.Fatal("ambiguous verification result")
		}
	}
}
