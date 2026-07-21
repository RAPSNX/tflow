// Package diag provides a minimal diagnostic-output seam for best-effort
// cleanup failures. It intentionally has no levels, no framework, and no
// buffering: it exists so that a best-effort cleanup failure is surfaced to
// the operator without replacing the original operation error that callers
// already return.
package diag

import (
	"fmt"
	"io"
	"os"
)

// Output is the destination for diagnostics emitted by Warnf. Tests may
// redirect it to capture output; production code leaves it at os.Stderr.
var Output io.Writer = os.Stderr

// Warnf emits a short diagnostic for a non-fatal, best-effort failure. It
// never returns an error and must not be used in place of returning the
// original operation error to the caller.
func Warnf(format string, args ...any) {
	fmt.Fprintf(Output, "tflow: "+format+"\n", args...)
}
