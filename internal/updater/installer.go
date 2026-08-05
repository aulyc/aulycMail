package updater

import (
	"context"
	"net/http"
)

type Installer interface {
	PrepareAndLaunch(context.Context, Manifest, *http.Client, func(string, int)) error
}

func NewInstaller() Installer { return newPlatformInstaller() }
