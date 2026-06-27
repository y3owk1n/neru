//go:build windows

// internal/core/infra/platform/windows/alert.go
// Native Windows alert dialogs using MessageBoxW.

package windows

import (
	"os/exec"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Alert choice constants that mirror the platform/factory.go values.
// These must match exactly so factory_windows.go can pass them through.
const (
	ConfigOnboardingCreate   = 1
	ConfigOnboardingDefaults = 2
	ConfigOnboardingQuit     = 3

	ConfigValidationOK       = 1
	ConfigValidationCopyPath = 2
)

const (
	mbOK            = 0x00000000
	mbOKCancel      = 0x00000001
	mbYesNoCancel   = 0x00000003
	mbIconWarning   = 0x00000030
	mbIconInfo      = 0x00000040
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
	titlePtr, _ := syscall.UTF16PtrFromString(title)
	messagePtr, _ := syscall.UTF16PtrFromString(message)
	if titlePtr == nil || messagePtr == nil {
		return 0
	}

	// MB_TOPMOST | MB_SETFOREGROUND ensures the dialog appears above other windows.
	flags := mbType | mbTopmost | mbSetForeground

	ret, _, _ := procMessageBox.Call(
		0,
		uintptr(unsafe.Pointer(messagePtr)),
		uintptr(unsafe.Pointer(titlePtr)),
		flags,
	)

	return int(ret)
}

// copyToClipboard copies text to the Windows clipboard via the clip.exe utility.
func copyToClipboard(text string) {
	cmd := exec.Command("clip")
	cmd.Stdin = strings.NewReader(text)
	_ = cmd.Run()
}

// ShowConfigValidationErrorAlert displays a config validation error dialog.
func ShowConfigValidationErrorAlert(errorMessage, configPath string) int {
	title := "Neru - Configuration Validation Failed"
	message := "Neru encountered an error while loading your configuration file:\n\n" +
		errorMessage + "\n\nConfig file: " + configPath +
		"\n\nClick OK to exit, or Cancel to copy the path."

	result := showMessageBox(title, message, mbOKCancel|mbIconWarning)
	if result == idCancel {
		copyToClipboard(configPath)

		return ConfigValidationCopyPath
	}

	return ConfigValidationOK
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
		return ConfigOnboardingCreate
	case idNo:
		return ConfigOnboardingDefaults
	case idCancel:
		return ConfigOnboardingQuit
	default:
		return ConfigOnboardingDefaults
	}
}

// ShowAlert displays a simple alert dialog with an OK button.
func ShowAlert(title, message string) {
	showMessageBox(title, message, mbOK|mbIconWarning)
}
