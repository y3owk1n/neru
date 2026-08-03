// Package overlay implements ports.OverlayPort over the platform overlay
// backends.
//
// Adapter wraps a manager.Interface and converts between domain models and the
// render models the backends draw. The overlay vocabulary the application layer
// names — Mode, ManagerInterface, StateChange, NoOpManager — is aliased here
// from the manager package.
//
// The backends are subpackages: manager holds the contract, and darwin, linux
// and windows implement it. Init and Get build and reach the process-wide
// manager, which is one native window per process.
//
// The render subpackages own what is drawn inside it: hints, grid,
// recursive-grid cells, the mode and sticky-modifier indicators, and the
// virtual pointer.
package overlay
