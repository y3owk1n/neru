// Package mouse implements ports.MousePort. The rules every platform shares
// live in adapter.go, and each platform supplies only its injection functions,
// in backend_darwin.go, backend_linux.go, backend_windows.go and
// backend_other.go.
package mouse
