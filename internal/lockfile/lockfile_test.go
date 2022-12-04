package lockfile

import (
	"bytes"
	"os"
	"testing"
)

func TestLockfile(t *testing.T) {
	dir := t.TempDir()
	inFile := "../../testdata/ex-lock.jsonl"
	outFile := dir + "/ex-lock.jsonl"
	// load test file
	l, err := LoadFile(inFile)
	if err != nil {
		t.Errorf("failed to load lockfile: %v", err)
		return
	}
	// verify content
	entry, err := l.Get("Test", "X")
	if err != nil {
		t.Errorf("failed to get test/X entry: %v", err)
		return
	}
	if entry.Version != "123" {
		t.Errorf("version mismatch, expected 123, received %s", entry.Version)
	}
	// write file
	err = SaveFile(outFile, l)
	if err != nil {
		t.Errorf("failed to save lockfile: %v", err)
		return
	}
	// verify file contains the same content
	bIn, err := os.ReadFile(inFile)
	if err != nil {
		t.Errorf("failed to read %s: %v", inFile, err)
		return
	}
	bOut, err := os.ReadFile(outFile)
	if err != nil {
		t.Errorf("failed to read %s: %v", outFile, err)
		return
	}
	if !bytes.Equal(bIn, bOut) {
		t.Errorf("output does not match input file %s:\n%s", inFile, bOut)
	}
}
