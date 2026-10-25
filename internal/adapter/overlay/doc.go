// Package overlay implements ports.OverlayPort over the platform overlay
// backends. Adapter wraps a manager.Interface and converts domain models into
// the render models the backends draw. The manager subpackage holds the
// contract, the darwin, linux and windows subpackages implement it, and the
// render subpackages own what is drawn.
// Init and Get build and reach the process-wide manager.
package overlay
