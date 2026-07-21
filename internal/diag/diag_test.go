package diag

import (
	"bytes"
	"strings"
	"testing"
)

func TestWarnfWritesToOutput(t *testing.T) {
	var buf bytes.Buffer
	original := Output
	Output = &buf
	defer func() { Output = original }()

	Warnf("kill orphaned session %q: %v", "tflow-p-abc", "boom")

	got := buf.String()
	if !strings.HasPrefix(got, "tflow: ") {
		t.Fatalf("Warnf output = %q, want tflow: prefix", got)
	}
	if !strings.Contains(got, `kill orphaned session "tflow-p-abc": boom`) {
		t.Fatalf("Warnf output = %q, want formatted message", got)
	}
	if !strings.HasSuffix(got, "\n") {
		t.Fatalf("Warnf output = %q, want trailing newline", got)
	}
}
