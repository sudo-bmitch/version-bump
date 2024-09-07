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
	// write file, including unused entries
	err = l.SaveFile(outFile, false)
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
	// add a new entry
	err = l.Set("Test", "Z", "123")
	if err != nil {
		t.Errorf("failed to set Z: %v", err)
	}
	// overwrite same entry
	err = l.Set("Test", "Z", "789")
	if err != nil {
		t.Errorf("failed to set Z: %v", err)
	}
	// write file, without unused entries
	err = l.SaveFile(outFile, true)
	if err != nil {
		t.Errorf("failed to save lockfile: %v", err)
		return
	}
	// read file of used entries
	lUsed, err := LoadFile(outFile)
	if err != nil {
		t.Errorf("failed to load lockfile: %v", err)
		return
	}
	// verify X is included but Y is missing
	entry, err = lUsed.Get("Test", "X")
	if err != nil {
		t.Errorf("failed to get test/X entry: %v", err)
		return
	}
	if entry.Version != "123" {
		t.Errorf("version mismatch, expected 123, received %s", entry.Version)
	}
	entry, err = lUsed.Get("Test", "Z")
	if err != nil {
		t.Errorf("failed to get test/Z entry: %v", err)
		return
	}
	if entry.Version != "789" {
		t.Errorf("version mismatch, expected 789, received %s", entry.Version)
	}
	_, err = lUsed.Get("Test", "Y")
	if err == nil {
		t.Errorf("did not fail when reading an unused entry")
	}
}

func TestNil(t *testing.T) {
	var l *Locks
	err := l.Set("A", "B", "C")
	if err == nil {
		t.Errorf("Set succeeded")
	}
	_, err = l.Get("A", "B")
	if err == nil {
		t.Errorf("Get succeeded")
	}
	err = l.Save(false)
	if err == nil {
		t.Errorf("Save succeeded")
	}
	err = l.SaveFile("./test.json", false)
	if err == nil {
		t.Errorf("SaveFile succeeded")
	}
	b := bytes.NewBuffer([]byte{})
	err = l.SaveWriter(b, false)
	if err == nil {
		t.Errorf("SaveWriter succeeded")
	}
}
