//go:build windows

package cli

import (
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/y3owk1n/neru/internal/derrors"
)

// Service management on Windows is a Task Scheduler task with a logon trigger,
// registered for the current user. A Run key would start the daemon at login
// too, but it cannot say whether the daemon is running, stop it, or start it
// again — the four verbs besides install and uninstall — so the scheduler is
// what gives `status`, `start`, `stop` and `restart` something real to mean.
//
// CGO is off on Windows, so the scheduler is driven through raw COM vtable
// calls in the shape internal/adapter/accessibility/native/windows uses for UI
// Automation. The task is described as XML and handed to
// ITaskFolder::RegisterTask whole, which keeps the COM surface to three
// objects: the service, its root folder, and the registered task.
const (
	// serviceTaskPath is the task's full path in the scheduler, which is also
	// what a user types into `schtasks /TN` or finds in Task Scheduler's own
	// window, so it stays the plain program name under the root folder.
	serviceTaskPath = `\Neru`

	// serviceTaskMarker is written into the task's description, and is the
	// only positive evidence that a \Neru task is Neru's to stop and delete —
	// a user or a deployment tool can register a task by the same name.
	serviceTaskMarker = "Installed by `neru services install`"
)

// serviceTaskTemplate is the Task Scheduler definition Neru registers, in the
// schema `schtasks /XML` and the Task Scheduler window both read.
//
// The logon trigger and the principal both name the current user, so the task
// starts with that user's session and only that one. ExecutionTimeLimit PT0S
// lifts the scheduler's default three-day limit on a running task, which would
// otherwise kill the daemon on the fourth day. Priority 4 is the normal process
// class with the main thread above normal; the scheduler's own default, 7, is
// below-normal, and latency is the product. RestartOnFailure is the counterpart
// of launchd's KeepAlive and systemd's Restart=on-failure.
const serviceTaskTemplate = `<?xml version="1.0" encoding="UTF-16"?>
<Task version="1.2" xmlns="http://schemas.microsoft.com/windows/2004/02/mit/task">
  <RegistrationInfo>
    <Description>Neru keyboard-driven mouse replacement daemon. ` + serviceTaskMarker + `; Neru rewrites this task on install and deletes it on uninstall.</Description>
    <URI>` + serviceTaskPath + `</URI>
  </RegistrationInfo>
  <Triggers>
    <LogonTrigger>
      <Enabled>true</Enabled>
      <UserId>NERU_USER_ID</UserId>
    </LogonTrigger>
  </Triggers>
  <Principals>
    <Principal id="Author">
      <UserId>NERU_USER_ID</UserId>
      <LogonType>InteractiveToken</LogonType>
      <RunLevel>LeastPrivilege</RunLevel>
    </Principal>
  </Principals>
  <Settings>
    <MultipleInstancesPolicy>IgnoreNew</MultipleInstancesPolicy>
    <DisallowStartIfOnBatteries>false</DisallowStartIfOnBatteries>
    <StopIfGoingOnBatteries>false</StopIfGoingOnBatteries>
    <AllowHardTerminate>true</AllowHardTerminate>
    <StartWhenAvailable>false</StartWhenAvailable>
    <RunOnlyIfNetworkAvailable>false</RunOnlyIfNetworkAvailable>
    <IdleSettings>
      <StopOnIdleEnd>false</StopOnIdleEnd>
      <RestartOnIdle>false</RestartOnIdle>
    </IdleSettings>
    <AllowStartOnDemand>true</AllowStartOnDemand>
    <Enabled>true</Enabled>
    <Hidden>false</Hidden>
    <RunOnlyIfIdle>false</RunOnlyIfIdle>
    <WakeToRun>false</WakeToRun>
    <ExecutionTimeLimit>PT0S</ExecutionTimeLimit>
    <Priority>4</Priority>
    <RestartOnFailure>
      <Interval>PT1M</Interval>
      <Count>3</Count>
    </RestartOnFailure>
  </Settings>
  <Actions Context="Author">
    <Exec>
      <Command>NERU_BINARY_PATH</Command>
      <Arguments>launch</Arguments>
    </Exec>
  </Actions>
</Task>`

// xmlTextEscaper escapes what cannot appear literally inside an XML element. A
// directory called "A&B" is a legal place for neru.exe to live, and an
// unescaped one is a task the scheduler refuses to register.
var xmlTextEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")

// renderServiceTask fills the task template with the binary that is installing
// itself and the SID of the user it runs as.
func renderServiceTask(binaryPath, userSID string) string {
	return strings.NewReplacer(
		"NERU_BINARY_PATH", xmlTextEscaper.Replace(binaryPath),
		"NERU_USER_ID", xmlTextEscaper.Replace(userSID),
	).Replace(serviceTaskTemplate)
}

// getBinaryPath resolves the running binary to a real path, because the task
// outlives whatever PATH or link the install was run through.
func getBinaryPath() (string, error) {
	execPath, err := os.Executable()
	if err != nil {
		return "", derrors.Wrap(err, derrors.CodeConfigIOFailed, "failed to locate the neru binary")
	}

	resolved, err := filepath.EvalSymlinks(execPath)
	if err != nil {
		return "", derrors.Wrap(
			err,
			derrors.CodeConfigIOFailed,
			"failed to resolve the neru binary path",
		)
	}

	return resolved, nil
}

// currentUserSID is the principal the task runs as. The SID rather than the
// account name, so a renamed account keeps its task.
func currentUserSID() (string, error) {
	current, err := user.Current()
	if err != nil {
		return "", derrors.Wrap(err, derrors.CodeInternal, "failed to look up the current user")
	}

	return current.Uid, nil
}

// COM plumbing for the Task Scheduler 2.0 API (taskschd.h).
var (
	modole32    = windows.NewLazySystemDLL("ole32.dll")
	modoleaut32 = windows.NewLazySystemDLL("oleaut32.dll")

	procCoInitializeEx   = modole32.NewProc("CoInitializeEx")
	procCoUninitialize   = modole32.NewProc("CoUninitialize")
	procCoCreateInstance = modole32.NewProc("CoCreateInstance")
	procSysFreeString    = modoleaut32.NewProc("SysFreeString")

	clsidTaskScheduler = guidMust("{0F87369F-A4E5-4CFC-BD3E-73E6154572DD}")
	iidITaskService    = guidMust("{2FABA4C7-4DA9-4013-9697-20CC3FD40F85}")
)

const (
	coinitMultithreaded = 0x0
	clsctxInprocServer  = 0x1

	hresultSOK         = 0
	hresultSFalse      = 1
	hresultChangedMode = 0x80010106 // RPC_E_CHANGED_MODE
	hresultNotFound    = 0x80070002 // HRESULT_FROM_WIN32(ERROR_FILE_NOT_FOUND)

	// TASK_CREATE refuses to replace a task that already exists, which install
	// checks for itself first so it can say who owns the existing one.
	taskCreate = 0x2
	// TASK_LOGON_INTERACTIVE_TOKEN: run in the logged-on user's session, with
	// the token of that session, which is the only one a desktop daemon can use.
	taskLogonInteractiveToken = 0x3

	// TASK_STATE values from get_State.
	taskStateUnknown  = 0
	taskStateDisabled = 1
	taskStateQueued   = 2
	taskStateReady    = 3
	taskStateRunning  = 4

	// Vtable slots. IUnknown holds 0–2 and IDispatch 3–6 on every interface
	// here; the rest follow the order in taskschd.h.
	vtRelease = 2

	vtServiceGetFolder = 7
	vtServiceConnect   = 10

	vtFolderGetTask      = 13
	vtFolderDeleteTask   = 15
	vtFolderRegisterTask = 16

	vtTaskGetState   = 9
	vtTaskGetEnabled = 10
	vtTaskRun        = 12
	vtTaskGetXML     = 20
	vtTaskStop       = 23
)

func guidMust(value string) windows.GUID {
	guid, err := windows.GUIDFromString(value)
	if err != nil {
		panic(err)
	}

	return guid
}

// variant is a VARIANT laid out for 64-bit Windows. Every VARIANT the
// scheduler takes here is VT_EMPTY, and every one is passed by value, which on
// x64 and ARM64 means a pointer to a copy: the struct is larger than a
// register, so both ABIs pass it by reference. The copy lives in a variable
// the caller owns across the call and keeps alive after it, so the collector
// cannot take it back while COM reads it.
type variant struct {
	vt       uint16
	reserved [3]uint16
	value    [2]uintptr
}

func (v *variant) ptr() uintptr {
	return uintptr(unsafe.Pointer(v))
}

// bstr is a BSTR built in Go memory for the scheduler to read: a four-byte
// length prefix, the UTF-16 text, and a NUL terminator, with the pointer
// handed out aimed at the text. Every string that crosses into COM here is
// copied by the callee before the call returns, so nothing needs SysAllocString
// or a matching free; the buffer only has to stay alive across the call, which
// is what the runtime.KeepAlive after each call is for.
type bstr struct {
	buf []uint16
}

const (
	// bstrPrefixUnits is the four-byte length prefix measured in UTF-16 units.
	bstrPrefixUnits = 2
	utf16UnitBytes  = 2
	bitsPerUint16   = 16
)

func newBSTR(value string) (bstr, error) {
	chars, err := windows.UTF16FromString(value)
	if err != nil {
		return bstr{}, derrors.Wrap(
			err,
			derrors.CodeInvalidInput,
			"failed to encode a string for the Task Scheduler",
		)
	}

	byteLen := uint32(len(chars)-1) * utf16UnitBytes // excludes the terminator
	buf := make([]uint16, bstrPrefixUnits+len(chars))
	buf[0] = uint16(byteLen)
	buf[1] = uint16(byteLen >> bitsPerUint16)
	copy(buf[bstrPrefixUnits:], chars)

	return bstr{buf: buf}, nil
}

func (b bstr) ptr() uintptr {
	return uintptr(unsafe.Pointer(&b.buf[bstrPrefixUnits]))
}

// readBSTR copies a BSTR the scheduler allocated into a Go string and frees it.
func readBSTR(value *uint16) string {
	if value == nil {
		return ""
	}

	text := windows.UTF16PtrToString(value)
	discardCall(procSysFreeString.Call(uintptr(unsafe.Pointer(value))))

	return text
}

// discardCall consumes a fire-and-forget COM call result; the release and
// free calls it sinks have no actionable failure path.
func discardCall(uintptr, uintptr, error) {}

func comCall(this unsafe.Pointer, index int, args ...uintptr) uintptr {
	vtbl := *(*unsafe.Pointer)(this)
	method := *(*uintptr)(unsafe.Add(vtbl, uintptr(index)*unsafe.Sizeof(uintptr(0))))

	full := make([]uintptr, 0, len(args)+1)
	full = append(full, uintptr(this))
	full = append(full, args...)

	ret, _, _ := syscall.SyscallN(method, full...)

	return ret
}

func failed(hresult uintptr) bool {
	return int32(hresult) < 0
}

func release(object unsafe.Pointer) {
	if object != nil {
		comCall(object, vtRelease)
	}
}

// hresultError turns a failed COM call into an error that quotes the system's
// own explanation. FormatMessage knows the scheduler's SCHED_E_* codes as well
// as the Win32 ones, so "The task XML is malformed" reaches the user rather
// than a bare number — and the number is kept for the ones it does not know.
func hresultError(attempt string, hresult uintptr) error {
	return derrors.Newf(
		derrors.CodeExecFailed,
		"failed to %s: %s (HRESULT 0x%08X)",
		attempt,
		windows.Errno(uint32(hresult)).Error(),
		uint32(hresult),
	)
}

// taskFolder is a connected ITaskService's root folder, which every subcommand
// works in. It lives only inside withTaskFolder, on one locked OS thread.
type taskFolder struct {
	folder unsafe.Pointer
}

// withTaskFolder connects to the scheduler and runs body against its root
// folder. All COM work happens on a single locked OS thread — initialize,
// create, connect, body, release — and nothing COM escapes the closure.
func withTaskFolder(body func(taskFolder) error) error {
	runtime.LockOSThread()

	defer runtime.UnlockOSThread()

	hresult, _, _ := procCoInitializeEx.Call(0, coinitMultithreaded)

	// S_OK and S_FALSE mean this call owns initialization on the thread and
	// must balance it with CoUninitialize. RPC_E_CHANGED_MODE means COM is
	// already up in another mode; leave it alone.
	switch uint32(hresult) {
	case hresultSOK, hresultSFalse:
		defer func() { discardCall(procCoUninitialize.Call()) }()
	case hresultChangedMode:
	default:
		return hresultError("initialize COM", hresult)
	}

	var service unsafe.Pointer

	hresult, _, _ = procCoCreateInstance.Call(
		uintptr(unsafe.Pointer(&clsidTaskScheduler)),
		0,
		clsctxInprocServer,
		uintptr(unsafe.Pointer(&iidITaskService)),
		uintptr(unsafe.Pointer(&service)),
	)
	if failed(hresult) || service == nil {
		return hresultError("create the Task Scheduler service", hresult)
	}
	defer release(service)

	// Connect(serverName, user, domain, password), all empty: the local
	// machine as the current user.
	var connectArgs [4]variant

	hresult = comCall(
		service,
		vtServiceConnect,
		connectArgs[0].ptr(),
		connectArgs[1].ptr(),
		connectArgs[2].ptr(),
		connectArgs[3].ptr(),
	)
	runtime.KeepAlive(&connectArgs)

	if failed(hresult) {
		return hresultError("connect to the Task Scheduler service", hresult)
	}

	rootPath, err := newBSTR(`\`)
	if err != nil {
		return err
	}

	var folder unsafe.Pointer

	hresult = comCall(
		service,
		vtServiceGetFolder,
		rootPath.ptr(),
		uintptr(unsafe.Pointer(&folder)),
	)
	runtime.KeepAlive(rootPath)

	if failed(hresult) || folder == nil {
		return hresultError("open the Task Scheduler root folder", hresult)
	}
	defer release(folder)

	return body(taskFolder{folder: folder})
}

// getTask returns the registered task at path, and whether there is one. The
// caller releases a found task.
func (f taskFolder) getTask(path string) (unsafe.Pointer, bool, error) {
	taskPath, err := newBSTR(path)
	if err != nil {
		return nil, false, err
	}

	var task unsafe.Pointer

	hresult := comCall(f.folder, vtFolderGetTask, taskPath.ptr(), uintptr(unsafe.Pointer(&task)))
	runtime.KeepAlive(taskPath)

	if uint32(hresult) == hresultNotFound {
		return nil, false, nil
	}

	if failed(hresult) || task == nil {
		return nil, false, hresultError("look up the Task Scheduler task "+path, hresult)
	}

	return task, true, nil
}

// registerTask registers a new task at path from its XML definition, refusing
// to replace one that exists.
func (f taskFolder) registerTask(path, xml string) error {
	taskPath, err := newBSTR(path)
	if err != nil {
		return err
	}

	definition, err := newBSTR(xml)
	if err != nil {
		return err
	}

	var (
		task     unsafe.Pointer
		userID   variant
		password variant
		sddl     variant
	)

	// RegisterTask(path, xmlText, flags, userId, password, logonType, sddl,
	// ppTask). The user and password stay empty because the definition's own
	// principal names the user, and an interactive token needs no password.
	hresult := comCall(
		f.folder,
		vtFolderRegisterTask,
		taskPath.ptr(),
		definition.ptr(),
		taskCreate,
		userID.ptr(),
		password.ptr(),
		taskLogonInteractiveToken,
		sddl.ptr(),
		uintptr(unsafe.Pointer(&task)),
	)
	runtime.KeepAlive(taskPath)
	runtime.KeepAlive(definition)
	runtime.KeepAlive(&userID)
	runtime.KeepAlive(&password)
	runtime.KeepAlive(&sddl)

	if failed(hresult) {
		return hresultError("register the Task Scheduler task "+path, hresult)
	}

	release(task)

	return nil
}

func (f taskFolder) deleteTask(path string) error {
	taskPath, err := newBSTR(path)
	if err != nil {
		return err
	}

	hresult := comCall(f.folder, vtFolderDeleteTask, taskPath.ptr(), 0)
	runtime.KeepAlive(taskPath)

	if failed(hresult) {
		return hresultError("delete the Task Scheduler task "+path, hresult)
	}

	return nil
}

// taskState reads IRegisteredTask::get_State.
func taskState(task unsafe.Pointer) (int32, error) {
	var state int32

	hresult := comCall(task, vtTaskGetState, uintptr(unsafe.Pointer(&state)))
	if failed(hresult) {
		return taskStateUnknown, hresultError("read the task state", hresult)
	}

	return state, nil
}

// taskEnabled reads IRegisteredTask::get_Enabled, a VARIANT_BOOL.
func taskEnabled(task unsafe.Pointer) (bool, error) {
	var enabled int16

	hresult := comCall(task, vtTaskGetEnabled, uintptr(unsafe.Pointer(&enabled)))
	if failed(hresult) {
		return false, hresultError("read whether the task is enabled", hresult)
	}

	return enabled != 0, nil
}

// taskXML reads IRegisteredTask::get_Xml, the definition as registered.
func taskXML(task unsafe.Pointer) (string, error) {
	var xml *uint16

	hresult := comCall(task, vtTaskGetXML, uintptr(unsafe.Pointer(&xml)))
	if failed(hresult) {
		return "", hresultError("read the task definition", hresult)
	}

	return readBSTR(xml), nil
}

// runTask is IRegisteredTask::Run, which starts an instance now regardless of
// the trigger.
func runTask(task unsafe.Pointer) error {
	var (
		running unsafe.Pointer
		params  variant
	)

	hresult := comCall(task, vtTaskRun, params.ptr(), uintptr(unsafe.Pointer(&running)))
	runtime.KeepAlive(&params)

	if failed(hresult) {
		return hresultError("start the task", hresult)
	}

	release(running)

	return nil
}

// stopTask is IRegisteredTask::Stop, which ends every running instance. It
// succeeds on a task with no instance, so stop is idempotent.
func stopTask(task unsafe.Pointer) error {
	hresult := comCall(task, vtTaskStop, 0)
	if failed(hresult) {
		return hresultError("stop the task", hresult)
	}

	return nil
}

// requireOwnTask stops uninstall from stopping or deleting a \Neru task
// somebody else registered: a deployment tool, or a user who set one up by
// hand before this feature existed. The path says nothing about who wrote it,
// so ownership is read out of the definition — the marker line Neru writes into
// the description is the one thing only an install puts there.
func requireOwnTask(task unsafe.Pointer, path string) error {
	xml, err := taskXML(task)
	if err != nil {
		return err
	}

	if !strings.Contains(xml, serviceTaskMarker) {
		return derrors.Newf(
			derrors.CodeInvalidInput,
			"the Task Scheduler task %s carries no %q line in its description, so Neru "+
				"did not register it; remove it in Task Scheduler or with `schtasks /Delete "+
				"/TN Neru`, and `neru services install` will register Neru's own in its place",
			path,
			serviceTaskMarker,
		)
	}

	return nil
}

func installServiceTask(path string) error {
	binPath, err := getBinaryPath()
	if err != nil {
		return err
	}

	userSID, err := currentUserSID()
	if err != nil {
		return err
	}

	return withTaskFolder(func(folder taskFolder) error {
		existing, found, err := folder.getTask(path)
		if err != nil {
			return err
		}

		if found {
			release(existing)

			return derrors.Newf(
				derrors.CodeInvalidInput,
				"a Task Scheduler task %s already exists; run `neru services uninstall` "+
					"— or remove it in Task Scheduler if something else registered it — "+
					"before installing again",
				path,
			)
		}

		err = folder.registerTask(path, renderServiceTask(binPath, userSID))
		if err != nil {
			return err
		}

		// Registered but never started is a task that blocks the next install
		// and runs nothing until the next login, so a start failure undoes the
		// registration, the way install on the other platforms undoes a unit
		// systemd or launchd would not load.
		task, found, err := folder.getTask(path)
		if err != nil {
			_ = folder.deleteTask(path)

			return err
		}

		if !found {
			_ = folder.deleteTask(path)

			return derrors.New(
				derrors.CodeInternal,
				"the Task Scheduler task "+path+" was registered but cannot be found",
			)
		}

		defer release(task)

		err = runTask(task)
		if err != nil {
			_ = folder.deleteTask(path)

			return err
		}

		return nil
	})
}

func uninstallServiceTask(path string) error {
	return withTaskFolder(func(folder taskFolder) error {
		task, found, err := folder.getTask(path)
		if err != nil {
			return err
		}

		// Nothing to uninstall is not a failure.
		if !found {
			return nil
		}

		defer release(task)

		err = requireOwnTask(task, path)
		if err != nil {
			return err
		}

		// Reported rather than swallowed: a stop that fails leaves the daemon
		// running while the CLI would otherwise print success.
		err = stopTask(task)
		if err != nil {
			return err
		}

		return folder.deleteTask(path)
	})
}

// errNotInstalled is what start, stop and restart say when there is no task to
// drive, which systemctl and launchctl each say in their own words.
func errNotInstalled(action, path string) error {
	return derrors.Newf(
		derrors.CodeInvalidInput,
		"services %s: no Task Scheduler task at %s — run `neru services install` first",
		action,
		path,
	)
}

// driveServiceTask runs verb against the installed task, or refuses when there
// is none.
func driveServiceTask(action, path string, verb func(unsafe.Pointer) error) error {
	return withTaskFolder(func(folder taskFolder) error {
		task, found, err := folder.getTask(path)
		if err != nil {
			return err
		}

		if !found {
			return errNotInstalled(action, path)
		}

		defer release(task)

		return verb(task)
	})
}

func restartTask(task unsafe.Pointer) error {
	err := stopTask(task)
	if err != nil {
		return err
	}

	return runTask(task)
}

func installService() error {
	return installServiceTask(serviceTaskPath)
}

func uninstallService() error {
	return uninstallServiceTask(serviceTaskPath)
}

func startService() error {
	return driveServiceTask("start", serviceTaskPath, runTask)
}

func stopService() error {
	return driveServiceTask("stop", serviceTaskPath, stopTask)
}

func restartService() error {
	return driveServiceTask("restart", serviceTaskPath, restartTask)
}

// serviceTaskState is everything `neru services status` reports, gathered
// before any of it is put into words, so the wording can be tested without a
// scheduler to ask.
type serviceTaskState struct {
	installed bool
	// path is where Neru registers its task, whether or not it is there yet.
	path string
	// state is the scheduler's TASK_STATE. enabled is the task's Enabled flag
	// and triggerEnabled the logon trigger's own, which Task Scheduler's window
	// lets a person switch off separately; both have to hold for the task to
	// start at login.
	state          int32
	enabled        bool
	triggerEnabled bool
}

// logonTriggerEnabled reads the logon trigger's enabled state out of a task
// definition. A trigger with no <Enabled> element is enabled, and a task with
// no logon trigger at all starts at login under no circumstances.
func logonTriggerEnabled(xml string) bool {
	_, trigger, found := strings.Cut(xml, "<LogonTrigger")
	if !found {
		return false
	}

	trigger, _, _ = strings.Cut(trigger, "</LogonTrigger>")

	return !strings.Contains(trigger, "<Enabled>false</Enabled>")
}

// The scheduler's states in the words its own window uses, with "ready"
// glossed because to a person asking whether the daemon is up it means no.
const (
	taskWordRunning  = "running"
	taskWordReady    = "ready (not running)"
	taskWordQueued   = "queued"
	taskWordDisabled = "disabled"
	taskWordUnknown  = "unknown"
)

func taskStateWord(state int32) string {
	switch state {
	case taskStateRunning:
		return taskWordRunning
	case taskStateReady:
		return taskWordReady
	case taskStateQueued:
		return taskWordQueued
	case taskStateDisabled:
		return taskWordDisabled
	default:
		return taskWordUnknown
	}
}

// describeServiceStatus puts a gathered state into one line. A machine with no
// task registered is an ordinary answer with the next step attached, not an
// error.
func describeServiceStatus(state serviceTaskState) string {
	if !state.installed {
		return "Service not installed: no Task Scheduler task at " + state.path +
			" — run `neru services install` to create it"
	}

	enabled := "enabled"
	if !state.enabled || !state.triggerEnabled {
		enabled = taskWordDisabled
	}

	return "Service installed: " + taskStateWord(state.state) + ", " + enabled + " at login"
}

func statusServiceTask(path string) string {
	state := serviceTaskState{path: path}

	err := withTaskFolder(func(folder taskFolder) error {
		task, found, err := folder.getTask(path)
		if err != nil {
			return err
		}

		if !found {
			return nil
		}

		defer release(task)

		state.installed = true

		state.state, err = taskState(task)
		if err != nil {
			return err
		}

		state.enabled, err = taskEnabled(task)
		if err != nil {
			return err
		}

		xml, err := taskXML(task)
		if err != nil {
			return err
		}

		state.triggerEnabled = logonTriggerEnabled(xml)

		return nil
	})
	if err != nil {
		return "Service status unavailable: " + err.Error()
	}

	return describeServiceStatus(state)
}

func statusService() string {
	return statusServiceTask(serviceTaskPath)
}
