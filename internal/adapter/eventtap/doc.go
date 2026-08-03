// Package eventtap implements ports.EventTapPort over the platform keyboard
// capture backends.
//
// Adapter wraps a tap.Tap and adds the state the port needs but a backend does
// not track — whether capture is currently enabled, and whether an overlay may
// let the keyboard through.
//
// The backends are subpackages: tap holds the contract, and darwin, linux and
// windows implement it. factory_<os>.go is the only place that knows which one
// exists.
package eventtap
