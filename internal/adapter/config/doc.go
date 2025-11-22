// Package config provides an adapter for configuration management.
//
// This package implements the ports.ConfigPort interface by wrapping
// the config.Service. It provides a thin adapter layer between the
// application services and the configuration infrastructure.
//
// # Usage
//
//	adapter := config.NewAdapter(configService)
//	cfg := adapter.Get()
package config
