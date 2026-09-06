package cli

import (
	"github.com/y3owk1n/neru/internal/adapter/platform"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/config/loader"
)

// clientConfigLoader builds the loader a client-side command reads the
// configuration with: defaults, no daemon, no logger, no alert dialog. The
// commands that judge a file (`config validate`, `doctor`, `roles explain`)
// share it so they read the same file the same way.
//
// The backend limit is wired here as it is in the daemon's two roots, so
// `config validate` and `doctor` warn about the same words the daemon does.
func clientConfigLoader() *loader.Service {
	svc := loader.NewService(config.DefaultConfig(), "", nil, nil)

	if platform.CurrentProfile().DisplayServer == platform.DisplayServerX11 {
		svc = svc.WithBackendInert(config.X11InertWords)
	}

	return svc
}
