package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/Rotlerxd/pipnest/internal/backends"
)

var (
	ErrNoBackends        = errors.New("no backends configured")
	ErrBackendNotFound   = errors.New("backend not found")
	ErrEmptyPackageName  = errors.New("package name is required")
	defaultBackendPolicy = []string{"uv", "pip"}
)

// Service orchestrates application package use-cases over pluggable backends.
type Service struct {
	mu       sync.RWMutex
	backends map[string]backends.Backend
	active   string
}

// New detects installed backends (uv/pip), builds the backend list and picks active backend by policy uv -> pip.
// Returns ErrNoBackends when neither uv nor pip is installed.
func New(pythonPath string) (*Service, error) {
	backendsByName := detectInstalledBackends(pythonPath)
	return newFromBackends(backendsByName)
}

func newFromBackends(backendsByName map[string]backends.Backend) (*Service, error) {
	if len(backendsByName) == 0 {
		return nil, ErrNoBackends
	}

	active := ""
	for _, name := range defaultBackendPolicy {
		if _, ok := backendsByName[name]; ok {
			active = name
			break
		}
	}

	if active == "" {
		names := make([]string, 0, len(backendsByName))
		for name := range backendsByName {
			names = append(names, name)
		}
		sort.Strings(names)
		active = names[0]
	}

	return &Service{
		backends: backendsByName,
		active:   active,
	}, nil
}

func (s *Service) getCurrentBackend() (backends.Backend, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	backend, ok := s.backends[s.active]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrBackendNotFound, s.active)
	}

	return backend, nil
}

// AvailableBackends returns sorted backend names known to the service.
func (s *Service) AvailableBackends() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	names := make([]string, 0, len(s.backends))
	for name := range s.backends {
		names = append(names, name)
	}
	sort.Strings(names)

	return names
}

// ActiveBackend returns the currently selected backend name.
func (s *Service) ActiveBackend() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.active
}

// SetActiveBackend switches active backend if it is configured.
func (s *Service) SetActiveBackend(name string) error {
	trimmedName := strings.TrimSpace(name)

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.backends[trimmedName]; !ok {
		return fmt.Errorf("%w: %q", ErrBackendNotFound, trimmedName)
	}

	s.active = trimmedName
	return nil
}

func validatePackageName(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", ErrEmptyPackageName
	}

	return trimmed, nil
}

func (s *Service) InstallPackage(ctx context.Context, packageName string) error {
	name, err := validatePackageName(packageName)
	if err != nil {
		return err
	}

	backend, err := s.getCurrentBackend()
	if err != nil {
		return err
	}

	return backend.InstallPackage(ctx, name)
}

func (s *Service) UninstallPackage(ctx context.Context, packageName string) error {
	name, err := validatePackageName(packageName)
	if err != nil {
		return err
	}

	backend, err := s.getCurrentBackend()
	if err != nil {
		return err
	}

	return backend.UninstallPackage(ctx, name)
}

func (s *Service) ShowPackage(ctx context.Context, packageName string) (backends.PackageDetails, error) {
	name, err := validatePackageName(packageName)
	if err != nil {
		return backends.PackageDetails{}, err
	}

	backend, err := s.getCurrentBackend()
	if err != nil {
		return backends.PackageDetails{}, err
	}

	return backend.ShowPackage(ctx, name)
}

func (s *Service) ListPackages(ctx context.Context) ([]backends.Package, error) {
	backend, err := s.getCurrentBackend()
	if err != nil {
		return nil, err
	}

	return backend.ListPackages(ctx)
}
