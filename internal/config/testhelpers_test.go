package config_test

// Role entries below are written in the semantic vocabulary, the same way user
// configuration is. Tests that assert on resolved output run both the input and
// the expectation through the resolver so they hold on every platform.
const (
	TestRoleButton     = "button"
	TestRoleTextField  = "text_field"
	TestRoleLink       = "link"
	TestRoleTabGroup   = "ax:AXTabGroup"
	TestRoleWebArea    = "ax:AXWebArea"
	TestBundleIDSafari = "com.apple.Safari"
	KeyCmdSpace        = "Cmd+Space"
	TestShellBash      = "/bin/bash"
	TestShellZsh       = "/bin/zsh"
	TestShellFlag      = "-lc"
	TestWindowsCmd     = `C:\Windows\System32\cmd.exe`
	TestGOOSLinux      = "linux"
	KeySuperSpace      = "Super+Space"
)
