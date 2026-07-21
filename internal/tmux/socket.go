package tmux

import (
	"os"
	"path/filepath"
	"strings"
)

// socketArgs returns the tmux CLI arguments that select the tflow server.
//
// tmux resolves "-L tflow" by joining TMUX_TMPDIR (or a default) with the
// socket name, which only finds the running server if that directory still
// matches the one the server was started under. If the current process is
// already inside a tflow session, $TMUX names the exact socket path the
// running server is listening on ("<path>,<pid>,<session>"); when that
// path's basename is the tflow socket name, addressing it directly with
// "-S <path>" is reliable regardless of TMUX_TMPDIR drift. Otherwise fall
// back to "-L tflow" for discovery via the default socket directory.
func socketArgs() []string {
	if path, ok := tflowSocketPathFromEnv(os.Getenv("TMUX")); ok {
		return []string{"-S", path}
	}
	return []string{"-L", socketName}
}

func tflowSocketPathFromEnv(tmuxEnv string) (string, bool) {
	path, _, _ := strings.Cut(tmuxEnv, ",")
	if path == "" {
		return "", false
	}
	if filepath.Base(path) != socketName {
		return "", false
	}
	return path, true
}
