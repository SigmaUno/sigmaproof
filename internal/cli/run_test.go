package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/SigmaUno/sigmaproof/pkg/proof"
	"github.com/SigmaUno/sigmaproof/pkg/proof/commitment"
)

func TestLocalWorkflowAndHonestExitCodes(t *testing.T) {
	dir := t.TempDir()
	doc := filepath.Join(dir, "document.txt")
	pkg := filepath.Join(dir, "document.sigmaproof")
	if err := os.WriteFile(doc, []byte("synthetic document"), 0600); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	if code := Run([]string{"create", "--unanchored", "--representation", "original", doc, pkg}, &out, &errOut); code != 0 {
		t.Fatalf("create %d: %s", code, errOut.String())
	}
	before, _ := os.ReadFile(pkg)
	info, err := os.Stat(pkg)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("private file permissions not preserved")
	}
	for _, offline := range []bool{false, true} {
		out.Reset()
		errOut.Reset()
		args := []string{"verify", "--json"}
		want := 3
		if offline {
			args = append(args, "--offline")
			want = 0
		}
		args = append(args, doc, pkg)
		if code := Run(args, &out, &errOut); code != want {
			t.Fatalf("verify %d want %d: %s", code, want, errOut.String())
		}
		var report proof.Report
		if err := json.Unmarshal(out.Bytes(), &report); err != nil {
			t.Fatal(err)
		}
		if !report.IntegrityVerified() || report.Anchor.Status != proof.Unavailable {
			t.Fatal("misleading verification report")
		}
	}
	if code := Run([]string{"create", "--unanchored", "--representation", "original", doc, pkg}, &out, &errOut); code != 1 {
		t.Fatal("overwrote existing package")
	}
	after, _ := os.ReadFile(pkg)
	if !bytes.Equal(before, after) {
		t.Fatal("existing package changed")
	}
	if err := os.WriteFile(doc, []byte("altered"), 0600); err != nil {
		t.Fatal(err)
	}
	if code := Run([]string{"verify", "--offline", doc, pkg}, &out, &errOut); code != 1 {
		t.Fatal("accepted modified document")
	}
	leftovers, _ := filepath.Glob(filepath.Join(dir, ".sigmaproof-*"))
	if len(leftovers) != 0 {
		t.Fatal("temporary private data left behind")
	}
}

func TestUsageAndMissingFiles(t *testing.T) {
	for _, args := range [][]string{{"verify"}, {"verify", "--remote"}, {"create", "doc", "proof"}, {"create", "--unanchored", "--representation", "invalid", "doc", "proof"}} {
		var out, errOut bytes.Buffer
		if code := Run(args, &out, &errOut); code != 2 {
			t.Fatalf("%v exit %d", args, code)
		}
	}
	var out, errOut bytes.Buffer
	if code := Run([]string{"verify", "--offline", "missing.pdf", "missing.sigmaproof"}, &out, &errOut); code != 1 {
		t.Fatalf("missing input exit %d", code)
	}
}

func TestCreationRefusesSymlinkAndLeavesNoPartialFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	link := filepath.Join(dir, "link")
	if err := os.WriteFile(target, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if err := writePrivateFile(link, []byte("replace")); err == nil {
		t.Fatal("followed destination symlink")
	}
	got, _ := os.ReadFile(target)
	if string(got) != "keep" {
		t.Fatal("overwrote symlink target")
	}
	doc := filepath.Join(dir, "doc")
	pkg := filepath.Join(dir, "pkg")
	os.WriteFile(doc, []byte("oversized"), 0600)
	var out, errOut bytes.Buffer
	if code := Run([]string{"create", "--unanchored", "--representation", "derived", "--max-document-bytes", "1", doc, pkg}, &out, &errOut); code != 1 {
		t.Fatal("ignored document limit")
	}
	if _, err := os.Stat(pkg); !os.IsNotExist(err) {
		t.Fatal("created output after failure")
	}
}

func TestUnsupportedAndMalformedPackageExits(t *testing.T) {
	dir := t.TempDir()
	doc := filepath.Join(dir, "doc")
	file := filepath.Join(dir, "pkg")
	if err := os.WriteFile(doc, nil, 0600); err != nil {
		t.Fatal(err)
	}
	p, err := proof.CreateUnanchored(bytes.NewReader(nil), commitment.Original, 0)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := p.MarshalBinary()
	var out, errOut bytes.Buffer
	for _, tc := range []struct {
		data []byte
		want int
	}{
		{append(append([]byte{}, data...), 0), 1},
		{append([]byte("SIGMAPRF\xff"), data[9:]...), 3},
	} {
		if err := os.WriteFile(file, tc.data, 0600); err != nil {
			t.Fatal(err)
		}
		if got := Run([]string{"verify", "--offline", "--json", doc, file}, &out, &errOut); got != tc.want {
			t.Fatalf("got %d want %d", got, tc.want)
		}
	}
}
