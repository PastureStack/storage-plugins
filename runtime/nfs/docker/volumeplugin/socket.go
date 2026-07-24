package volumeplugin

import (
	"os"
	"os/exec"
	"path/filepath"
)

// The following two directory need to be bind-mounted from host
const (
	dockerSockDir         = "/run/docker/plugins"
	controlPlaneSocketDir = "/var/run/rancher/storage"
)

func ForceSymlinkInDockerPlugins(driver string) error {
	if err := os.MkdirAll(dockerSockDir, 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(controlPlaneSocketDir, 0755); err != nil {
		return err
	}
	symlinkFile := filepath.Join(dockerSockDir, driver+".sock")

	return exec.Command("ln", "-sf", ControlPlaneSocketFile(driver), symlinkFile).Run()
}

func ControlPlaneSocketFile(driver string) string {
	return filepath.Join(controlPlaneSocketDir, driver+".sock")
}
