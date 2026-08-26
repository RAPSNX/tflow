package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/rapsnx/tflow/internal/buildinfo"
)

func TestRunStartsTflowWithoutCommand(t *testing.T) {
	started := false
	if err := run(nil, &bytes.Buffer{}, func() error {
		started = true
		return nil
	}); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if !started {
		t.Fatal("run() did not start tflow")
	}
}

func TestRunHelp(t *testing.T) {
	for _, args := range [][]string{{"help"}, {"-h"}, {"--help"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var output bytes.Buffer
			if err := run(args, &output, func() error {
				t.Fatal("help must not start tflow")
				return nil
			}); err != nil {
				t.Fatalf("run(%q) error = %v", args, err)
			}
			if got := output.String(); got != helpText {
				t.Fatalf("help output = %q, want %q", got, helpText)
			}
		})
	}
}

func TestRunVersion(t *testing.T) {
	for _, args := range [][]string{{"version"}, {"--version"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var output bytes.Buffer
			if err := run(args, &output, func() error {
				t.Fatal("version must not start tflow")
				return nil
			}); err != nil {
				t.Fatalf("run(%q) error = %v", args, err)
			}
			want := "tflow " + buildinfo.Version() + "\n"
			if got := output.String(); got != want {
				t.Fatalf("version output = %q, want %q", got, want)
			}
		})
	}
}

func TestRunUnknownCommand(t *testing.T) {
	err := run([]string{"flow"}, &bytes.Buffer{}, func() error {
		t.Fatal("unknown command must not start tflow")
		return nil
	})
	if err == nil || err.Error() != "unknown command \"flow\"; run \"tflow --help\" for usage" {
		t.Fatalf("run() error = %v", err)
	}
}

func TestRunCreateWorkerRequiresPayload(t *testing.T) {
	err := run([]string{"create-worker"}, &bytes.Buffer{}, func() error {
		t.Fatal("create-worker must not start tflow")
		return nil
	})
	if err == nil || err.Error() != "create-worker requires one payload" {
		t.Fatalf("run() error = %v", err)
	}
}
