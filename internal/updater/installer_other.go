//go:build !darwin

package updater

import (
	"context"
	"fmt"
	"net/http"
)

type platformInstaller struct{}

func newPlatformInstaller() Installer { return &platformInstaller{} }

func (p *platformInstaller) PrepareAndLaunch(context.Context, Manifest, *http.Client, func(string, int)) error {
	return fmt.Errorf("automatic update installation is only available on macOS")
}
