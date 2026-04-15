package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/Rotlerxd/pipnest/internal/backends"
	"github.com/Rotlerxd/pipnest/internal/venv"
)

var (
	ErrNoBackends        = errors.New("no backends configured")
	ErrBackendNotFound   = errors.New("backend not found")
	ErrEmptyPackageName  = errors.New("package name is required")
	defaultBackendPolicy = []string{"uv", "pip"}
)

// Service orchestrates application package use-cases over pluggable backends.
type Service struct {
	mu            sync.RWMutex
	backends      map[string]backends.Backend
	venvManager   venv.Manager
	activeBackend string
	activeVenv    *venv.Venv
	strategy      venv.VenvCreationStrategy
}

// NewService New detects installed backends (uv/pip), builds the backend list and picks active backend by policy uv -> pip.
// Returns ErrNoBackends when neither uv nor pip is installed.
func NewService(pythonPath string) (*Service, error) {
	backendsByName := detectInstalledBackends(pythonPath)
	return newFromBackends(backendsByName)
}

func newFromBackends(backendsByName map[string]backends.Backend) (*Service, error) {
	if len(backendsByName) == 0 {
		return nil, ErrNoBackends
	}

	activeBackend := ""
	for _, name := range defaultBackendPolicy {
		if _, ok := backendsByName[name]; ok {
			activeBackend = name
			break
		}
	}

	if activeBackend == "" {
		names := make([]string, 0, len(backendsByName))
		for name := range backendsByName {
			names = append(names, name)
		}
		sort.Strings(names)
		activeBackend = names[0]
	}

	return &Service{
		backends:      backendsByName,
		activeBackend: activeBackend,
		venvManager:   venv.NewVenvManager(),
	}, nil
}

func (s *Service) getCurrentBackend() (backends.Backend, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	backend, ok := s.backends[s.activeBackend]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrBackendNotFound, s.activeBackend)
	}

	return backend, nil
}

// SetActiveBackend switches active backend if it is configured.
func (s *Service) SetActiveBackend(name string) error {
	trimmedName := strings.TrimSpace(name)

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.backends[trimmedName]; !ok {
		return fmt.Errorf("%w: %q", ErrBackendNotFound, trimmedName)
	}

	s.activeBackend = trimmedName
	return nil
}

func (s *Service) GetAvailableBackends() (map[string]backends.Backend, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	returnedBackends := make(map[string]backends.Backend, len(s.backends))
	for name := range s.backends {
		returnedBackends[name] = s.backends[name]
	}

	return returnedBackends, nil
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

// NewVenvManager creates a venv manager with strategy selected by active package backend.
func (s *Service) NewVenvManager() *venv.VenvManager {
	return venv.NewVenvManager()
}

func (s *Service) ListVenv(ctx context.Context) ([]venv.Venv, error) {
	return s.venvManager.ListVenvs(ctx)
}

func (s *Service) CreateVenv(ctx context.Context, input venv.CreateVenvInput) (*venv.Venv, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	createdVenv, err := s.venvManager.CreateVenv(ctx, s.strategy, input)
	if err != nil {
		return nil, err
	}

	return &createdVenv, nil
}

func (s *Service) setVenvCreationStrategy(strategy venv.VenvCreationStrategy) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	s.strategy = strategy
}

func (s *Service) SetVenv(ctx context.Context, venvToSet *venv.Venv) error {
	if venvToSet == nil {
		return errors.New("venv to set cannot be nil")
	}

	availableVenvs, err := s.ListVenv(ctx)
	if err != nil {
		return err
	}
	found := false
	for _, v := range availableVenvs {
		if venv.EqualsVenv(&v, venvToSet) {
			found = true
			break
		}
	}

	if !found {
		return errors.New("venv not found")
	}

	s.activeVenv = venvToSet
	return nil
}

func (s *Service) GetCurrentVenv() *venv.Venv {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.activeVenv
}
