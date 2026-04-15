package venv

// Venv describes a managed virtual environment.
type Venv struct {
	Name       string
	Path       string
	PythonPath string
}

func EqualsVenv(a, b *Venv) bool {
	return a.Path == b.Path && a.Name == b.Name
}

// CreateVenvInput contains parameters required to create a virtual environment.
type CreateVenvInput struct {
	Name       string
	Path       string
	PythonPath string
}
