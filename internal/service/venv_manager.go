package service

import "context"

// Venv describes a managed virtual environment.
type Venv struct {
	Name       string
	Path       string
	PythonPath string
	IsActive   bool
}

// CreateVenvInput contains parameters required to create a virtual environment.
type CreateVenvInput struct {
	Name       string
	Path       string
	PythonPath string
}

// VenvManager defines service-layer operations for managing virtual environments.
type VenvManager interface {
	ListVenvs(ctx context.Context) ([]Venv, error)
	CreateVenv(ctx context.Context, input CreateVenvInput) (Venv, error)
	DeleteVenv(ctx context.Context, path string) error
	SelectVenv(ctx context.Context, path string) error
	CurrentVenv(ctx context.Context) (Venv, error)
}
