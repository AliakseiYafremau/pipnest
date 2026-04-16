package backends

import "context"

// Package represents an installed package.
type Package struct {
	Name    string
	Version string
}

// PackageDetails represents result of show-package operation.
type PackageDetails struct {
	Name     string
	Version  string
	Summary  string
	HomePage string
	Author   string
	License  string
	Raw      string
}

// Backend defines base package-manager operations shared by pip/uv backends.
type Backend interface {
	SetPythonPath(string)
	InstallPackage(ctx context.Context, packageName string) error
	UninstallPackage(ctx context.Context, packageName string) error
	ShowPackage(ctx context.Context, packageName string) (PackageDetails, error)
	ListPackages(ctx context.Context) ([]Package, error)
}
