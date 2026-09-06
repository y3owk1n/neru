package cli

import (
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/config/loader"
)

// clientConfigLoader builds the loader a client-side command reads the
// configuration with: defaults, no daemon, no logger, no alert dialog. The
// commands that judge a file (`config validate`, `doctor`, `roles explain`)
// share it so they read the same file the same way.
func clientConfigLoader() *loader.Service {
	return loader.NewService(config.DefaultConfig(), "", nil, nil)
}
