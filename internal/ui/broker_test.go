package ui

import (
	"bytes"
	"testing"
)

func TestAppendHistoryKeepsLatestBytes(t *testing.T) {
	var proc sessionProcess

	proc.appendHistory(bytes.Repeat([]byte("a"), maxSessionHistoryBytes-4))
	proc.appendHistory([]byte("bcdefghi"))

	if proc.history.Len() != maxSessionHistoryBytes {
		t.Fatalf("history.Len() = %d, want %d", proc.history.Len(), maxSessionHistoryBytes)
	}
	if !bytes.HasSuffix(proc.history.Bytes(), []byte("bcdefghi")) {
		t.Fatalf("history suffix = %q, want %q", proc.history.Bytes()[proc.history.Len()-8:], "bcdefghi")
	}
}
