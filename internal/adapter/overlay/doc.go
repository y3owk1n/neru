// Package overlay implements ports.OverlayPort over the platform overlay
// backends. Adapter wraps a manager.Interface and converts domain models into
// the render models the backends draw. manager holds the contract; darwin,
// linux and windows implement it; the render subpackages own what is drawn.
// Init and Get build and reach the process-wide manager.
package overlay
