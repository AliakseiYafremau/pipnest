package pip

import (
	"context"
	"strings"

	"github.com/Rotlerxd/pipnest/internal/backends"
)

// Backend implements the shared package-manager contract using pip commands.
type Backend struct {
	Binary     string
	PythonPath string
}

// NewPipBackend creates a pip backend with optional binary and interpreter path.
func NewPipBackend(binary, pythonPath string) *Backend {
	if strings.TrimSpace(binary) == "" {
		binary = "pip"
	}

	return &Backend{
		Binary:     binary,
		PythonPath: strings.TrimSpace(pythonPath),
	}
}

var _ backends.Backend = (*Backend)(nil)

func (b *Backend) InstallPackage(ctx context.Context, packageName string) error {
	_, err := b.runPip(ctx, "install", packageName)
	return err
}

func (b *Backend) UninstallPackage(ctx context.Context, packageName string) error {
	_, err := b.runPip(ctx, "uninstall", "-y", packageName)
	return err
}

func (b *Backend) ShowPackage(ctx context.Context, packageName string) (backends.PackageDetails, error) {
	out, err := b.runPip(ctx, "show", packageName)
	if err != nil {
		return backends.PackageDetails{}, err
	}

	return parseShowOutput(out), nil
}

func (b *Backend) ListPackages(ctx context.Context) ([]backends.Package, error) {
	out, err := b.runPip(ctx, "list", "--format", "json")
	if err != nil {
		return nil, err
	}

	return parseListOutput(out)
}

// runPip composes and executes a pip command from provided args.
//
// Command composition order:
// 1) Start with operation args received by this method (for example: "install", "requests").
// 2) If PythonPath is set, prepend "--python <PythonPath>" so pip targets that interpreter.
// 3) Use Backend.Binary as the command executable (usually "pip").
//
// Example:
//   runPip(ctx, "install", "requests")
// becomes:
//   pip --python /path/to/python install requests
// when PythonPath is configured.
func (b *Backend) runPip(ctx context.Context, args ...string) (string, error) {
	cmdArgs := make([]string, 0, len(args)+2)
	// Prepend interpreter binding flags before operation args.
	if strings.TrimSpace(b.PythonPath) != "" {
		cmdArgs = append(cmdArgs, "--python", strings.TrimSpace(b.PythonPath))
	}
	// Append operation-specific arguments as passed by caller.
	cmdArgs = append(cmdArgs, args...)

	// Execute: <Binary> <cmdArgs...>
	return backends.DefaultRunner(ctx, b.Binary, cmdArgs...)
}
