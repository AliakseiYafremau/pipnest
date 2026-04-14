package service

import (
	"os/exec"

	"github.com/Rotlerxd/pipnest/internal/backends"
	"github.com/Rotlerxd/pipnest/internal/backends/pip"
	"github.com/Rotlerxd/pipnest/internal/backends/uv"
)

var lookupBinary = exec.LookPath

func detectInstalledBackends(pythonPath string) map[string]backends.Backend {
	available := make(map[string]backends.Backend, 2)

	if uvPath, err := lookupBinary("uv"); err == nil {
		available["uv"] = uv.NewUvBackend(uvPath, pythonPath)
	}

	if pipPath, err := lookupBinary("pip"); err == nil {
		available["pip"] = pip.NewPipBackend(pipPath, pythonPath)
	}

	return available
}
