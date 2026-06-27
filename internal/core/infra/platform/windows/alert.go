//go:build windows

// internal/core/infra/platform/windows/alert.go
// Native Windows alert dialogs using MessageBoxW.

package windows

import (
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	mbOK            = 0x00000000
	mbOKCancel      = 0x00000001
	mbYesNoCancel   = 0x00000003
	mbIconWarning   = 0x00000030
	mbIconInfo      = 0x00000040
	mbDefButton2    = 0x00000100
	mbDefButton3    = 0x00000200
	mbTopmost       = 0x00040000
	mbSetForeground = 0x00010000

	idOK     = 1
	idCancel = 2
	idYes    = 6
	idNo     = 7
)

var (
	user32Alert    = windows.NewLazySystemDLL("user32.dll")
	procMessageBox = user32Alert.NewProc("MessageBoxW")
)

// showMessageBox displays a Win32 MessageBoxW and returns the button ID.
func showMessageBox(title, message string, mbType uintptr) int {
	t, _ := syscall.UTF16PtrFromString(title)
	m, _ := syscall.UTF16PtrFromString(message)
	if t == nil || m == nil {
		return 0
	}

	// MB_TOPMOST | MB_SETFOREGROUND ensures the dialog appears above other windows.
	flags := mbType | mbTopmost | mbSetForeground

	ret, _, _ := procMessageBox.Call(
		0,
		uintptr(unsafe.Pointer(m)),
		uintptr(unsafe.Pointer(t)),
		flags,
	)
	return int(ret)
}

// ShowConfigValidationErrorAlert displays a config validation error dialog.
func ShowConfigValidationErrorAlert(errorMessage, configPath string) int {
	title := "Neru - Configuration Validation Failed"
	message := "Neru encountered an error while loading your configuration file:\n\n" +
		errorMessage + "\n\nConfig file: " + configPath +
		"\n\nClick OK to exit, or Cancel to copy the path."

	result := showMessageBox(title, message, mbOKCancel|mbIconWarning)
	switch result {
	case idOK:
		return 1
	case idCancel:
		return 2
	default:
		return 1
	}
}

// ShowConfigOnboardingAlert displays a config onboarding dialog for new users.
func ShowConfigOnboardingAlert(configPath string) int {
	title := "Neru - Welcome!"
	message := "No configuration file found.\n\n" +
		"A default config will be created at:\n" + configPath + "\n\n" +
		"You can run 'neru config init' later to recreate it.\n\n" +
		"Yes    = Create Config\n" +
		"No     = Use Defaults (No Config)\n" +
		"Cancel = Quit"

	result := showMessageBox(title, message, mbYesNoCancel|mbIconInfo)
	switch result {
	case idYes:
		return 1
	case idNo:
		return 2
	case idCancel:
		return 3
	default:
		return 2
	}
}

// ShowAlert displays a simple alert dialog with an OK button.
func ShowAlert(title, message string) {
	showMessageBox(title, message, mbOK|mbIconWarning)
}
