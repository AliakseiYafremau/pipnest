package requirements

import "context"

type Dependency struct {
	Name    string
	Version string
}


type PackageManager interface {
	Install(ctx context.Context, pkg_name string) error // pip install <pkg_name>
	InstallFromFile(ctx context.Context, file_path string) error // pip install -r <file_path>
	Freeze(ctx context.Context, file_path string) error // pip freeze > <file_path>
	List(ctx context.Context) ([]Dependency, error) // pip list
	Search(ctx context.Context, query string) ([]Dependency, error) // pip search <query>
	Remove(ctx context.Context, pkg_name string) error // pip uninstall <pkg_name>

	RunPython(ctx context.Context, code string) (string, error) // python -c "<code>"
}
