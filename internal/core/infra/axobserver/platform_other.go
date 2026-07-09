//go:build !darwin

package axobserver

import "unsafe"

// On non-darwin platforms the observer is a no-op. A push-based change-observer
// for AT-SPI (Linux) or UI Automation (Windows) can implement these shims later;
// the Manager and ObservationTarget types are already platform-agnostic.

func platformStartObserverThread() {}

func platformStopObserverThread() {}

func platformObserverThreadRunning() bool { return false }

func platformSetObserverMessagingTimeout(_ float64) {}

func platformArmObserver(_ int, _ uint64, _ uint32) unsafe.Pointer { return nil }

func platformDisarmObserver(_ unsafe.Pointer, _ bool) {}

func platformSetObserverSink(_ func(pid int, epoch uint64, notif string)) {}
