package service

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/Rotlerxd/pipnest/internal/backends"
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

func TestNew_SelectsUvByDefaultPolicy(t *testing.T) {
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

	svc, err := New("")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if got := svc.ActiveBackend(); got != "uv" {
		t.Fatalf("ActiveBackend() = %q, want %q", got, "uv")
	}
}

func TestNew_FallsBackToPipWhenUvMissing(t *testing.T) {
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

	svc, err := New("")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if got := svc.ActiveBackend(); got != "pip" {
		t.Fatalf("ActiveBackend() = %q, want %q", got, "pip")
	}
}

func TestNew_ReturnsErrorWhenNoInstalledBackends(t *testing.T) {
	originalLookup := lookupBinary
	t.Cleanup(func() { lookupBinary = originalLookup })

	lookupBinary = func(_ string) (string, error) {
		return "", fmt.Errorf("not found")
	}

	_, err := New("")
	if !errors.Is(err, ErrNoBackends) {
		t.Fatalf("expected ErrNoBackends, got %v", err)
	}
}

func TestNewFromBackends_SelectsUvByDefaultPolicy(t *testing.T) {
	uvBackend := &fakeBackend{}
	pipBackend := &fakeBackend{}

	svc, err := newFromBackends(map[string]backends.Backend{
		"pip": pipBackend,
		"uv":  uvBackend,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if got := svc.ActiveBackend(); got != "uv" {
		t.Fatalf("ActiveBackend() = %q, want %q", got, "uv")
	}
}

func TestNewFromBackends_FallsBackToPipWhenUvMissing(t *testing.T) {
	pipBackend := &fakeBackend{}

	svc, err := newFromBackends(map[string]backends.Backend{
		"pip": pipBackend,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if got := svc.ActiveBackend(); got != "pip" {
		t.Fatalf("ActiveBackend() = %q, want %q", got, "pip")
	}
}

func TestNewFromBackends_ReturnsErrorOnEmptyBackends(t *testing.T) {
	_, err := newFromBackends(map[string]backends.Backend{})
	if !errors.Is(err, ErrNoBackends) {
		t.Fatalf("expected ErrNoBackends, got %v", err)
	}
}

func TestSetActiveBackend(t *testing.T) {
	svc, err := newFromBackends(map[string]backends.Backend{
		"pip": &fakeBackend{},
		"uv":  &fakeBackend{},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := svc.SetActiveBackend("pip"); err != nil {
		t.Fatalf("SetActiveBackend() error = %v", err)
	}

	if got := svc.ActiveBackend(); got != "pip" {
		t.Fatalf("ActiveBackend() = %q, want %q", got, "pip")
	}

	err = svc.SetActiveBackend("poetry")
	if !errors.Is(err, ErrBackendNotFound) {
		t.Fatalf("expected ErrBackendNotFound, got %v", err)
	}
}

func TestAvailableBackends_ReturnsSortedNames(t *testing.T) {
	svc, err := newFromBackends(map[string]backends.Backend{
		"z": &fakeBackend{},
		"a": &fakeBackend{},
		"m": &fakeBackend{},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	got := svc.AvailableBackends()
	want := []string{"a", "m", "z"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("AvailableBackends() = %#v, want %#v", got, want)
	}
}

func TestDelegatesOperationsToActiveBackend(t *testing.T) {
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
	svc, err := newFromBackends(map[string]backends.Backend{
		"pip": &fakeBackend{},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	err = svc.InstallPackage(context.Background(), "   ")
	if !errors.Is(err, ErrEmptyPackageName) {
		t.Fatalf("expected ErrEmptyPackageName, got %v", err)
	}
}
