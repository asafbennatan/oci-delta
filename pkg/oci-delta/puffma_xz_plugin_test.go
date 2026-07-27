package ocidelta

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func xzCompress(t *testing.T, data []byte) []byte {
	t.Helper()

	if _, err := exec.LookPath("xz"); err != nil {
		t.Skip("xz binary not available")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "data.xz")
	cmd := exec.Command("xz", "-9", "-e", "-c")
	cmd.Stdin = bytes.NewReader(data)
	out, err := os.Create(path)
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	cmd.Stdout = out
	if err := cmd.Run(); err != nil {
		out.Close()
		t.Fatalf("xz compress: %v", err)
	}
	out.Close()

	compressed, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read compressed data: %v", err)
	}
	return compressed
}

func TestPuffmaXzPluginName(t *testing.T) {
	if got := (puffmaXzPlugin{}).Name(); got != puffmaXzPluginName {
		t.Errorf("Name() = %q, want %q", got, puffmaXzPluginName)
	}
}

func TestPuffmaXzPluginMayApply(t *testing.T) {
	p := puffmaXzPlugin{}
	tests := []struct {
		basenames []string
		want      bool
	}{
		{[]string{"module.ko.xz"}, true},
		{[]string{"a.txt", "b.xz"}, true},
		{[]string{"module.ko"}, false},
		{[]string{"archive.bz2"}, false},
		{nil, false},
	}
	for _, tt := range tests {
		if got := p.MayApply(tt.basenames); got != tt.want {
			t.Errorf("MayApply(%v) = %v, want %v", tt.basenames, got, tt.want)
		}
	}
}

func TestPuffmaXzPluginDiffApplyRoundTrip(t *testing.T) {
	oldContent := bytes.Repeat([]byte("the quick brown fox jumps over the lazy dog\n"), 200)
	newContent := append(append([]byte{}, oldContent...), []byte("one more line appended at the end\n")...)

	oldXz := xzCompress(t, oldContent)
	newXz := xzCompress(t, newContent)

	p := puffmaXzPlugin{}
	patch, err := p.Diff(oldXz, newXz)
	if err != nil {
		t.Fatalf("Diff failed: %v", err)
	}

	got, err := p.Apply(oldXz, patch)
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}
	if !bytes.Equal(got, newXz) {
		t.Fatalf("round trip mismatch: got %d bytes, want %d bytes", len(got), len(newXz))
	}
}

func TestPuffmaXzPluginDiffRejectsNonXzInput(t *testing.T) {
	p := puffmaXzPlugin{}
	if _, err := p.Diff([]byte("not xz"), []byte("also not xz")); err == nil {
		t.Fatal("expected error diffing non-xz data")
	}
}
