package loader

import (
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
)

// ResolveDerived turns the configuration a user wrote into the one the daemon
// runs on, for a caller that loaded it: config.ResolveDerived, plus the one
// derivation that belongs to loading rather than to the schema.
//
// Dropping a launcher binding for a mode that is switched off is a derivation
// like any other here — the user wrote the binding and the daemon runs without
// it — and it lives on this side because it is the loader that decides what a
// binding means. Anything that writes to the Config itself belongs in
// config.ResolveDerived, where the guardrail can see it.
func ResolveDerived(cfg *config.Config) {
	cfg.ResolveDerived()
	removeLauncherBindingsForDisabledModes(cfg)
}

// ApplyFieldChange applies one `neru config set` and hands back the pair a
// caller has to keep: the configuration to run on, and the one it was derived
// from.
//
// The change lands on the written configuration rather than the running one,
// because that is the only copy where "the user never set grid.row_labels" is
// still true. Setting grid.characters on the running config finds the labels
// already filled in and leaves them — labeling the grid from a character set
// it no longer uses — while setting them here leaves them empty for the
// resolution below to infer again. The same holds for every theme color
// derived from [theme].
//
// Neither input is modified: a field change can still fail validation upstream,
// and the configuration the daemon is running on has to survive that intact.
func ApplyFieldChange(
	written *config.Config,
	key, value string,
) (*config.Config, *config.Config, error) {
	newWritten, copyErr := DeepCopyConfig(written)
	if copyErr != nil {
		return nil, nil, derrors.Wrap(
			copyErr,
			derrors.CodeSerializationFailed,
			"copy the written configuration to change it",
		)
	}

	setErr := SetField(newWritten, key, value)
	if setErr != nil {
		return nil, nil, setErr
	}

	running, runningErr := DeepCopyConfig(newWritten)
	if runningErr != nil {
		return nil, nil, derrors.Wrap(
			runningErr,
			derrors.CodeSerializationFailed,
			"copy the written configuration to derive from it",
		)
	}

	ResolveDerived(running)

	return running, newWritten, nil
}

// derive settles a freshly layered configuration and reports the pair. It is
// the load path's use of ResolveDerived, and the reason the snapshot is a copy
// rather than the same pointer: the resolution below writes over the fields the
// snapshot exists to remember.
func derive(cfg *config.Config) (*config.Config, error) {
	written, copyErr := DeepCopyConfig(cfg)
	if copyErr != nil {
		return nil, derrors.Wrap(
			copyErr,
			derrors.CodeSerializationFailed,
			"snapshot the written configuration",
		)
	}

	ResolveDerived(cfg)

	return written, nil
}
