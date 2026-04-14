//go:build linux || darwin
// +build linux darwin

package uv

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/Rotlerxd/pipnest/internal/backends"
)

func TestRunUvPip_ComposesCommand(t *testing.T) {
	tests := []struct {
		name       string
		backend    *Backend
		runArgs    []string
		wantBinary string
		wantArgs   []string
	}{
		{
			name:       "without python path",
			backend:    &Backend{Binary: "uv"},
			runArgs:    []string{"install", "requests"},
			wantBinary: "uv",
			wantArgs:   []string{"pip", "install", "requests"},
		},
		{
			name:       "with python path",
			backend:    &Backend{Binary: "uv", PythonPath: "/venv/bin/python"},
			runArgs:    []string{"show", "requests"},
			wantBinary: "uv",
			wantArgs:   []string{"pip", "--python", "/venv/bin/python", "show", "requests"},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			originalRunner := backends.DefaultRunner
			t.Cleanup(func() { backends.DefaultRunner = originalRunner })

			var gotBinary string
			var gotArgs []string
			backends.DefaultRunner = func(_ context.Context, binary string, args ...string) (string, error) {
				gotBinary = binary
				gotArgs = append([]string{}, args...)
				return "ok", nil
			}

			// Act
			out, err := tc.backend.runUvPip(context.Background(), tc.runArgs...)

			// Assert
			if err != nil {
				t.Fatalf("runUvPip returned error: %v", err)
			}
			if out != "ok" {
				t.Fatalf("unexpected output: %q", out)
			}
			if gotBinary != tc.wantBinary {
				t.Fatalf("unexpected binary: %q", gotBinary)
			}
			if !reflect.DeepEqual(gotArgs, tc.wantArgs) {
				t.Fatalf("unexpected args\nwant: %#v\ngot:  %#v", tc.wantArgs, gotArgs)
			}
		})
	}
}

func TestRunUvPip_PropagatesRunnerError(t *testing.T) {
	// Arrange
	backend := &Backend{Binary: "uv"}

	originalRunner := backends.DefaultRunner
	t.Cleanup(func() { backends.DefaultRunner = originalRunner })

	wantErr := errors.New("runner failed")
	backends.DefaultRunner = func(_ context.Context, _ string, _ ...string) (string, error) {
		return "", wantErr
	}

	// Act
	_, err := backend.runUvPip(context.Background(), "list")

	// Assert
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected propagated error %v, got %v", wantErr, err)
	}
}
