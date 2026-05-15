package storage

import (
	"os"
	"testing"
)

func TestReadOnlyAfterClose(t *testing.T) {
	dir, err := os.MkdirTemp("", "odk-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	dbPath := dir + "/db"

	w, err := New(dbPath)
	if err != nil {
		t.Fatal("writer open:", err)
	}
	w.Close()

	r, err := NewReadOnly(dbPath)
	if err != nil {
		t.Fatal("readonly on closed db:", err)
	}
	r.Close()
}
