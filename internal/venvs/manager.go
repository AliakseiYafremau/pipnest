package venvs

import "context"

type Manager interface {
	Create(ctx context.Context, name string) error
	Remove(ctx context.Context, name string) error
	RunPython(ctx context.Context, name string, code string) (string, error)
}
