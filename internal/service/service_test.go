package service

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/Rotlerxd/pipnest/internal/backends"
	"github.com/Rotlerxd/pipnest/internal/venv"
)

type fakeBackend struct {
	installCalls   []string
	uninstallCalls []string
	showCalls      []string
	listCalls      int

	installErr   error
	uninstallErr error
	showResult   backends.PackageDetails
	showErr      error
	listResult   []backends.Package
	listErr      error
}

func (f *fakeBackend) SetPythonPath(string) {
	//TODO implement me
	panic("implement me")
}

func (f *fakeBackend) InstallPackage(_ context.Context, packageName string) error {
	f.installCalls = append(f.installCalls, packageName)
	return f.installErr
}

func (f *fakeBackend) UninstallPackage(_ context.Context, packageName string) error {
	f.uninstallCalls = append(f.uninstallCalls, packageName)
	return f.uninstallErr
}

func (f *fakeBackend) ShowPackage(_ context.Context, packageName string) (backends.PackageDetails, error) {
	f.showCalls = append(f.showCalls, packageName)
	return f.showResult, f.showErr
}

func (f *fakeBackend) ListPackages(_ context.Context) ([]backends.Package, error) {
	f.listCalls++
	return f.listResult, f.listErr
}

type fakeVenvManager struct {
	listResult []venv.Venv
	listErr    error
}

func (f *fakeVenvManager) ListVenvs(_ context.Context) ([]venv.Venv, error) {
	return f.listResult, f.listErr
}

func (f *fakeVenvManager) CreateVenv(_ context.Context, _ venv.VenvCreationStrategy, _ venv.CreateVenvInput) (venv.Venv, error) {
	return venv.Venv{}, errors.New("not implemented")
}

func TestNew_SelectsUvByDefaultPolicy(t *testing.T) {
	// Arrange
	originalLookup := lookupBinary
	t.Cleanup(func() { lookupBinary = originalLookup })

	lookupBinary = func(name string) (string, error) {
		switch name {
		case "uv":
			return "/usr/bin/uv", nil
		case "pip":
			return "/usr/bin/pip", nil
		default:
			return "", fmt.Errorf("not found")
		}
	}

	// Act
	_, err := NewService("")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Assert
	// NewService behavior is covered more precisely by newFromBackends tests.
}

func TestNew_FallsBackToPipWhenUvMissing(t *testing.T) {
	// Arrange
	originalLookup := lookupBinary
	t.Cleanup(func() { lookupBinary = originalLookup })

	lookupBinary = func(name string) (string, error) {
		switch name {
		case "uv":
			return "", fmt.Errorf("not found")
		case "pip":
			return "/usr/bin/pip", nil
		default:
			return "", fmt.Errorf("not found")
		}
	}

	// Act
	_, err := NewService("")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Assert
	// NewService behavior is covered more precisely by newFromBackends tests.
}

func TestNew_ReturnsErrorWhenNoInstalledBackends(t *testing.T) {
	// Arrange
	originalLookup := lookupBinary
	t.Cleanup(func() { lookupBinary = originalLookup })

	lookupBinary = func(_ string) (string, error) {
		return "", fmt.Errorf("not found")
	}

	// Act
	_, err := NewService("")

	// Assert
	if !errors.Is(err, ErrNoBackends) {
		t.Fatalf("expected ErrNoBackends, got %v", err)
	}
}

func TestNewFromBackends_SelectsUvByDefaultPolicy(t *testing.T) {
	// Arrange
	uvBackend := &fakeBackend{}
	pipBackend := &fakeBackend{}

	// Act
	svc, err := newFromBackends(map[string]backends.Backend{
		"pip": pipBackend,
		"uv":  uvBackend,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	_ = svc.InstallPackage(context.Background(), "requests")

	// Assert
	if !reflect.DeepEqual(uvBackend.installCalls, []string{"requests"}) {
		t.Fatalf("expected uv backend to be selected by default policy, installCalls=%#v", uvBackend.installCalls)
	}
	if len(pipBackend.installCalls) != 0 {
		t.Fatalf("pip backend should be inactive, installCalls=%#v", pipBackend.installCalls)
	}
}

func TestNewFromBackends_FallsBackToPipWhenUvMissing(t *testing.T) {
	// Arrange
	pipBackend := &fakeBackend{}

	// Act
	svc, err := newFromBackends(map[string]backends.Backend{
		"pip": pipBackend,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	_ = svc.InstallPackage(context.Background(), "requests")

	// Assert
	if !reflect.DeepEqual(pipBackend.installCalls, []string{"requests"}) {
		t.Fatalf("expected pip backend to be selected when uv missing, installCalls=%#v", pipBackend.installCalls)
	}
}

func TestNewFromBackends_ReturnsErrorOnEmptyBackends(t *testing.T) {
	// Arrange
	// Act
	_, err := newFromBackends(map[string]backends.Backend{})

	// Assert
	if !errors.Is(err, ErrNoBackends) {
		t.Fatalf("expected ErrNoBackends, got %v", err)
	}
}

func TestSetActiveBackend(t *testing.T) {
	// Arrange
	svc, err := newFromBackends(map[string]backends.Backend{
		"pip": &fakeBackend{},
		"uv":  &fakeBackend{},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Act
	if err := svc.SetActiveBackend("pip"); err != nil {
		t.Fatalf("SetActiveBackend() error = %v", err)
	}

	// ensure setting an unknown backend returns error
	err = svc.SetActiveBackend("poetry")

	// Assert
	if !errors.Is(err, ErrBackendNotFound) {
		t.Fatalf("expected ErrBackendNotFound, got %v", err)
	}
}

// TestAvailableBackends removed along with AvailableBackends method.

func TestDelegatesOperationsToActiveBackend(t *testing.T) {
	// Arrange
	ctx := context.Background()
	uvBackend := &fakeBackend{
		showResult: backends.PackageDetails{Name: "requests", Version: "2.0.0"},
		listResult: []backends.Package{{Name: "requests", Version: "2.0.0"}},
	}
	pipBackend := &fakeBackend{}

	svc, err := newFromBackends(map[string]backends.Backend{
		"pip": pipBackend,
		"uv":  uvBackend,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Act

	if err := svc.InstallPackage(ctx, " requests "); err != nil {
		t.Fatalf("InstallPackage() error = %v", err)
	}
	if err := svc.UninstallPackage(ctx, "requests"); err != nil {
		t.Fatalf("UninstallPackage() error = %v", err)
	}

	details, err := svc.ShowPackage(ctx, "requests")
	if err != nil {
		t.Fatalf("ShowPackage() error = %v", err)
	}
	if details.Name != "requests" {
		t.Fatalf("unexpected ShowPackage() result: %#v", details)
	}

	packages, err := svc.ListPackages(ctx)
	if err != nil {
		t.Fatalf("ListPackages() error = %v", err)
	}
	if len(packages) != 1 || packages[0].Name != "requests" {
		t.Fatalf("unexpected ListPackages() result: %#v", packages)
	}

	if !reflect.DeepEqual(uvBackend.installCalls, []string{"requests"}) {
		t.Fatalf("unexpected install calls: %#v", uvBackend.installCalls)
	}
	if !reflect.DeepEqual(uvBackend.uninstallCalls, []string{"requests"}) {
		t.Fatalf("unexpected uninstall calls: %#v", uvBackend.uninstallCalls)
	}
	if !reflect.DeepEqual(uvBackend.showCalls, []string{"requests"}) {
		t.Fatalf("unexpected show calls: %#v", uvBackend.showCalls)
	}
	if uvBackend.listCalls != 1 {
		t.Fatalf("unexpected list calls: %d", uvBackend.listCalls)
	}

	if len(pipBackend.installCalls)+len(pipBackend.uninstallCalls)+len(pipBackend.showCalls)+pipBackend.listCalls != 0 {
		t.Fatalf("inactive backend should not be called")
	}
}

func TestInstallPackage_ValidatesPackageName(t *testing.T) {
	// Arrange
	svc, err := newFromBackends(map[string]backends.Backend{
		"pip": &fakeBackend{},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Act
	err = svc.InstallPackage(context.Background(), "   ")

	// Assert
	if !errors.Is(err, ErrEmptyPackageName) {
		t.Fatalf("expected ErrEmptyPackageName, got %v", err)
	}
}

func TestListVenv_DelegatesToVenvManager(t *testing.T) {
	// Arrange
	svc, err := newFromBackends(map[string]backends.Backend{"pip": &fakeBackend{}})
	if err != nil {
		t.Fatalf("newFromBackends() error = %v", err)
	}
	svc.venvManager = &fakeVenvManager{listResult: []venv.Venv{{Name: "a", Path: "/tmp/a"}}}

	// Act
	vs, err := svc.ListVenv(context.Background())

	// Assert
	if err != nil {
		t.Fatalf("ListVenv() error = %v", err)
	}
	if !reflect.DeepEqual(vs, []venv.Venv{{Name: "a", Path: "/tmp/a"}}) {
		t.Fatalf("unexpected venvs: %#v", vs)
	}
}

func TestSetVenv_ValidatesMembershipAndSetsCurrent(t *testing.T) {
	// Arrange
	svc, err := newFromBackends(map[string]backends.Backend{"pip": &fakeBackend{}})
	if err != nil {
		t.Fatalf("newFromBackends() error = %v", err)
	}
	available := []venv.Venv{{Name: "a", Path: "/tmp/a"}, {Name: "b", Path: "/tmp/b"}}
	svc.venvManager = &fakeVenvManager{listResult: available}

	// Act
	if err := svc.SetVenv(context.Background(), nil); err == nil {
		// Assert
		t.Fatalf("expected error for nil venv")
	}

	// Act
	err = svc.SetVenv(context.Background(), &venv.Venv{Name: "c", Path: "/tmp/c"})
	if err == nil {
		// Assert
		t.Fatalf("expected error when venv not found")
	}

	// Act
	toSet := &venv.Venv{Name: "b", Path: "/tmp/b"}
	err = svc.SetVenv(context.Background(), toSet)

	// Assert
	if err != nil {
		t.Fatalf("SetVenv() error = %v", err)
	}
	if got := svc.GetCurrentVenv(); got != toSet {
		t.Fatalf("GetCurrentVenv() = %#v, want %#v", got, toSet)
	}
}
