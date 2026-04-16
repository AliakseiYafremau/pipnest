package pip

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/Rotlerxd/pipnest/internal/backends"
)

func TestRunPip_ComposesCommand(t *testing.T) {
	tests := []struct {
		name       string
		backend    *Backend
		runArgs    []string
		wantBinary string
		wantArgs   []string
	}{
		{
			name:       "without python path",
			backend:    &Backend{Binary: "pip"},
			runArgs:    []string{"install", "requests"},
			wantBinary: "pip",
			wantArgs:   []string{"install", "requests"},
		},
		{
			name:       "with python path",
			backend:    &Backend{Binary: "pip", PythonPath: "/venv/bin/python"},
			runArgs:    []string{"show", "requests"},
			wantBinary: "pip",
			wantArgs:   []string{"--python", "/venv/bin/python", "show", "requests"},
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
			out, err := tc.backend.runPip(context.Background(), tc.runArgs...)

			// Assert
			if err != nil {
				t.Fatalf("runPip returned error: %v", err)
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

func TestRunPip_PropagatesRunnerError(t *testing.T) {
	// Arrange
	backend := &Backend{Binary: "pip"}

	originalRunner := backends.DefaultRunner
	t.Cleanup(func() { backends.DefaultRunner = originalRunner })

	wantErr := errors.New("runner failed")
	backends.DefaultRunner = func(_ context.Context, _ string, _ ...string) (string, error) {
		return "", wantErr
	}

	// Act
	_, err := backend.runPip(context.Background(), "list")

	// Assert
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected propagated error %v, got %v", wantErr, err)
	}
}

func TestSetPythonPath_ReflectsInRunPip(t *testing.T) {
	// Arrange
	backend := &Backend{Binary: "pip"}

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
	backend.SetPythonPath("/venv/bin/python")
	out, err := backend.runPip(context.Background(), "show", "requests")

	// Assert
	if err != nil {
		t.Fatalf("runPip returned error: %v", err)
	}
	if out != "ok" {
		t.Fatalf("unexpected output: %q", out)
	}
	if gotBinary != "pip" {
		t.Fatalf("unexpected binary: %q", gotBinary)
	}
	wantArgs := []string{"--python", "/venv/bin/python", "show", "requests"}
	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("unexpected args\nwant: %#v\ngot:  %#v", wantArgs, gotArgs)
	}
}
