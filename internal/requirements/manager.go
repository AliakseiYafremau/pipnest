package requirements

import "context"

type Dependency struct {
	Name    string
	Version string
}

type Manager interface {
	Load(ctx context.Context, path string) ([]Dependency, error)
	Save(ctx context.Context, path string, dependencies []Dependency) error
	Add(ctx context.Context, path string, dependency Dependency) error
	Remove(ctx context.Context, path string, name string) error
}
