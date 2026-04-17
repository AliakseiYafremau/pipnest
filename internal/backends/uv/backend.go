package uv

import (
	"context"
	"strings"

	"github.com/Rotlerxd/pipnest/internal/backends"
)

// Backend implements the shared package-manager contract using uv pip commands.
type Backend struct {
	Binary     string
	PythonPath string
}

func (b *Backend) SetPythonPath(newPath string) {
	b.PythonPath = newPath
}

// NewUvBackend creates a uv backend with optional binary and interpreter path.
func NewUvBackend(binary, pythonPath string) *Backend {
	if strings.TrimSpace(binary) == "" {
		binary = "uv"
	}

	return &Backend{
		Binary:     binary,
		PythonPath: strings.TrimSpace(pythonPath),
	}
}

var _ backends.Backend = (*Backend)(nil)

func (b *Backend) InstallPackage(ctx context.Context, packageName string) error {
	_, err := b.runUvPip(ctx, "install", packageName)
	return err
}

func (b *Backend) UninstallPackage(ctx context.Context, packageName string) error {
	_, err := b.runUvPip(ctx, "uninstall", "-y", packageName)
	return err
}

func (b *Backend) ShowPackage(ctx context.Context, packageName string) (backends.PackageDetails, error) {
	out, err := b.runUvPip(ctx, "show", packageName)
	if err != nil {
		return backends.PackageDetails{}, err
	}

	return parseShowOutput(out), nil
}

func (b *Backend) ListPackages(ctx context.Context) ([]backends.Package, error) {
	out, err := b.runUvPip(ctx, "list", "--format", "json")
	if err != nil {
		return nil, err
	}

	return parseListOutput(out)
}

// runUvPip composes and executes a uv pip command from provided args.
//
// Command composition order:
// 1) Start with "pip" subcommand namespace.
// 2) Append operation args received by this method.
//
// Note: uv's pip command does not accept the same --python flag that
// standalone pip accepts. We must not inject "--python" here to avoid
// argument errors when invoking uv.
func (b *Backend) runUvPip(ctx context.Context, args ...string) (string, error) {
	cmdArgs := make([]string, 0, len(args)+1)
	cmdArgs = append(cmdArgs, "pip")

	cmdArgs = append(cmdArgs, args...)

	return backends.DefaultRunner(ctx, b.Binary, cmdArgs...)
}
