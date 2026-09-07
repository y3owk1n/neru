# The Wayland keyboard is a proxy

**Status**: accepted

On Wayland, Neru now holds every physical keyboard (`EVIOCGRAB`) for the
daemon's lifetime and re-emits everything it reads through one uinput keyboard
of its own, `neru-keyboard-proxy`. The compositor's libinput reads that device
and nothing else. Capturing keys for a mode is then a routing decision on the one
goroutine that reads the devices — the same shape as the macOS `CGEventTap`,
which is installed once and enabled with a flag — rather than a grab that has to
be acquired on every activation.

The grab it replaces was acquired per mode, and it could not be acquired
instantly. Grabbing a device while a key is held routes that key's release to
the grabber alone, so libinput never sees it, keeps the key down, and swallows
its next press (#1087). For the modifier of an activation chord that meant a
stuck Super under every application the user touched next. So the tap waited,
polling `EVIOCGKEY` every five milliseconds, for the chord to be physically
released before it grabbed, and while it waited every key the user typed went
to the focused application — and if the base key outlived the modifier it went
on waiting, up to half a second more, with the overlay holding exclusive
keyboard focus and dropping what it received. On top of that it rebuilt its xkb
state from a fresh Wayland connection on every activation, four roundtrips and a
keymap compile, and ran a second reader of the same devices beside the passive
global-hotkey listener, which then had to reconcile its picture of the keyboard
every time a mode had come and gone (`waylandEvdevGrabGeneration`,
`resyncModifiersFromKernel`). The mode appeared at once; it accepted input a
hundred milliseconds to six hundred later. "Latency is the product" is the
first property in the root guide, and this was the Linux path's largest cost.

With the proxy there is no such window, because there is nothing to wait for.
The whole design rests on one invariant, and the forward rule
(`eventtap/linux/evdev_forward.go`) is that invariant and nothing else: **a
release is re-emitted exactly when its press was.** A press is re-emitted
whenever no mode is capturing and withheld otherwise; its repeats and its
release follow it wherever it went. A mode that opens under its activation chord
therefore costs nothing: the chord's presses reached the compositor, so its
releases do too, and the compositor's picture of the keyboard is never wrong in
either direction. Everything the old design carried to repair that picture —
the pre-grab waits, `initialKeys`, the synthetic releases injected at shutdown,
the passive listener's reconciliation — is deleted rather than rewritten.

The invariant covers presses the proxy saw, so the proxy has to be there before
the first one. It is built when the daemon starts, not by the first mode, and a
device is never held with a key down: a keyboard found with a key down under a
fresh grab (a daemon, or a remapper, launched from a compositor binding with the
binding's modifier still held) is let go again at once and grabbed once it is
idle, polled ten milliseconds apart. That wait is paid once per device, never
per activation. It replaces the first version's seeds, which pushed a key found
held as an already-forwarded press so its release would be re-emitted, and that
was the second rejected option below in another guise: libinput drops a release
for a key it never saw pressed on the proxy, the modifier stays down on the
physical device, and on Hyprland, which merges modifier state across every
keyboard, every key typed afterwards carried Super. It surfaced first for a
kanata user on Hyprland with no `[hotkeys]`, where nothing built the proxy
until a mode did, under the binding that launched it.

It buys three more things the tap could not have. A matched hotkey chord is
withheld, so the focused application never sees the activation chord, which is
what every other platform's hotkey mechanism already does. Modifier passthrough
becomes "re-emit this press after all": the held modifiers go out with it and
stay the compositor's until they are physically released, with no re-tap per
auto-repeat and nothing to unwind when the mode ends. And a mode never sees the
activation chord's modifier, since the session counts only presses withheld for
it, so a hint label typed while Super is still coming up is the label.

## Considered options

- **Keep the on-demand grab and tune the waits.** Sharing one reader between
  the listener and the tap removes the xkb rebuild and the ioctl polls; waking on
  the release event instead of a timer removes the five-millisecond quantum;
  dropping the second wait removes the overlay grab. What none of it removes is
  the wait itself, which is the user's own chord-release time, fifty to two
  hundred milliseconds on every activation, with the keys typed inside it going
  to the wrong window. The wait is inherent to acquiring a grab under a held
  key, so this path cannot reach macOS.
- **Grab immediately and forward the pre-held releases synthetically.** The
  compositor's picture is repaired, but libinput's per-device state is not
  reachable from userspace: it still has the chord's modifier down on the
  physical device and swallows its next press, which for a compositor binding
  such as Super+Enter after a mode delivers a bare Enter to the focused
  application. Rejected for that failure alone.
- **The proxy, opt-in behind a config option.** An option would let a user keep
  the old behaviour, and every option is a cost the root guide says to pay only
  when no single default is right for everyone. There is no such user: the old
  path was slower, lost keys, and could still stick a modifier when its bounds
  expired. What the proxy changes for the compositor — one virtual keyboard in
  place of the physical ones — is documented instead.

## Consequences

- Wayland keyboard capture needs a writable `/dev/uinput`, the same udev rule
  scroll injection and `neru key` already need. Without it the proxy reads
  passively: hotkeys still match, the chord also reaches the application, and a
  mode falls back to the overlay's keyboard focus as before. Without readable
  `/dev/input` there is no proxy at all, also as before.
- The compositor sees `neru-keyboard-proxy` while the daemon runs. Per-device
  compositor settings apply to it; the physical keyboards go quiet. LED state
  the compositor sets on the proxy is carried back to the physical devices.
- A key remapper (kanata, keyd) that holds its input keyboards refuses Neru's
  grab and is left alone; its virtual output keyboard is captured instead,
  because it carries the user's keys.
- A remapper that starts while the daemon holds the keyboards, launched later,
  restarted, or started by systemd in the same instant, finds them held; kanata
  treats the failed grab as a warning and runs with no input until a node is
  created under `/dev/input`. So the arrival of a remapper's output device, a
  uinput keyboard from another process (sysfs files it directly under
  `devices/virtual/input`, `isVirtualInputNode`, pinned by
  `TestIsVirtualInputNode_OnlyAUinputDeviceIsVirtual`), yields every physical
  keyboard the capture holds (`yieldPhysicalKeyboards`): each is ungrabbed and
  revoked (`EVIOCREVOKE`) once no key on it is down, so a forwarded press is
  never parted from its release, and the revoke wakes its reader with ENODEV
  and leaves the fd dead rather than reusable until the reader closes it. Kanata
  grabs its inputs two seconds after creating its output device, its startup
  delay, so three seconds after yielding the capture adopts again whatever is
  still free, which is a keyboard the remapper did not want. The initial scan
  runs the same yield when it finds such a device, for a remapper still inside
  its delay. Keyd and `kanata --nodelay` grab in the instant they start, before
  their output device can be seen, and are started first. Any uinput keyboard
  from another process looks like a remapper's output, so one that is not (a
  key injector, a controller mapper) costs the same three seconds once, with
  keys reaching the compositor unremapped and hotkeys silent. A mode never
  pays it: a keyboard is not let go while a session is capturing keys, the
  release and a session's start both happen under the device lock so neither
  slips past the other, and a session asked for while a keyboard is yielded is
  refused (`TestEvdevProxy_StartSession_RefusesWhileAKeyboardIsYielded`), so
  the mode falls back to the overlay's keyboard focus as it does while a key
  is held.
- A remapper that exits releases its input keyboards in the instant its output
  device disappears, and the released keyboards are existing nodes that no
  inotify event announces. So a captured device vanishing starts a rescan of
  `/dev/input`, which adopts them; when the remapper's output device comes
  back, the yield above hands them over again.
- A remapper's device auto-detect takes any node that advertises keyboard
  keys. Kanata's excludes only a device named `kanata`, keyd's only its own
  virtual keyboard, so either grabs `neru-keyboard-proxy` when the daemon
  creates it, or on its own start when the daemon is already up. With the
  remapper's output grabbed here that is a loop: every key circles between
  the two processes and reaches the compositor from neither. Neru cannot
  change the remapper's filter, so the proxy probes its own nodes every half
  second from the run goroutine, a grab and ungrab no key can fall inside
  because that goroutine is the device's only writer, and fails open when
  another process holds one (`probeOwnDevices`, pinned by
  `TestEvdevProxy_ProbeOwnDevices_FailsOpenWhenAnotherProcessHoldsAProxy`; the
  probe against a real kernel is
  `TestProxyNode_HeldByAnother_SeesAGrabFromAnotherFd`). The remapper's
  output returns to the compositor, so the user's keys and remaps work; what
  is lost is capturing keys for a mode. Failing open is the one time the
  proxy lets a keyboard go without waiting for it to be idle, so a key down
  at that instant, whose press the proxy re-emitted, is released on the proxy
  keyboard first (`releaseForwarded`, pinned by
  `TestEvdevProxy_FailOpen_ReleasesEveryKeyItForwarded`): its physical
  release goes to the physical device, whose libinput never saw the press
  and drops it, and a key left down on the proxy would stay down for the
  daemon's lifetime, a Super that Hyprland merged into every key typed
  afterwards. The setup guide tells remapper users to exclude the `neru-`
  devices instead.
- A remapper's output keyboard also advertises relative motion and mouse
  buttons, so a key can move the pointer, and so does a receiver that exposes a
  mouse and a keyboard on one node. Grabbing such a device takes its motion, so
  the motion is given somewhere to go: a second uinput device,
  `neru-pointer-proxy`, created the first time such a keyboard is grabbed and
  never otherwise, carries every relative axis and the mouse buttons. Buttons
  are not keys to the forward rule, the matcher or a mode; they and the motion
  bypass all three (`isPointerButton`), and a button outside the mouse range is
  dropped rather than advertised, since a joystick or gamepad button on the
  pointer proxy would have udev class it as a joystick (`isMouseButton`). The
  proxy keyboard itself stays a pure keyboard for the same reason.
- A device that reports position axes (a keyboard with a built-in trackpad on
  the same node) is never grabbed: re-emitting touches would need its axis
  ranges and slots, so it keeps its keys out of Neru's reach, as X11's grab
  already leaves them. Only ABS_X, ABS_Y and the multitouch position axes
  count (`hasPointerAxes`); a Bluetooth keyboard's volume knob is an absolute
  axis too, and turning such a keyboard away left it uncaptured.
- Every keystroke on the system crosses the daemon while it runs. The run
  goroutine takes no lock the mode handler can hold, and a crashed daemon is
  recovered by the kernel, which drops every grab and the uinput device with the
  process. A daemon that hangs rather than crashes holds the keyboards; that is
  the risk keyd's users accept, and the reason the run goroutine must stay as
  lean as the macOS tap callback.
