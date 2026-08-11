# Parity is measured in words, not subsystems

**Status**: accepted

Linux is labelled beta — "good for daily driving", with what is missing sitting
"around the edges rather than in your way." The list of those edges is
[Known Gaps](../CROSS_PLATFORM.md#known-gaps), and the roadmap sends every
contributor to it rather than keeping a second copy, "so the status you read is
the status the code reports." A sweep of the tree against that list found it was
neither correct nor complete. Two of its seven entries are not gaps: secure
input detection is a macOS concept with no X11 or Wayland counterpart, and GNOME
Wayland is a supported-desktop decision rather than a missing capability. Eight
real divergences were absent from it entirely — among them `neru services`,
which installs a launchd plist on macOS and returns `CodeNotSupported` from
every subcommand on Linux for want of a systemd unit; `neru docs`, which is
`CodeNotSupported` on Linux while `systray/open_other.go` in the same repo
already shells out to `xdg-open`; and `ShowNotification`, whose Linux body is
empty where its sibling `ShowAlert` at least refuses out loud.

The drift has one cause, and it is not neglect. The list was kept at the
granularity of the capability matrix, and the matrix asks the wrong question.
It asks whether a subsystem works. `smooth_scroll` is a cross-platform block in
the config schema that only `platform/darwin/scroll_animator.go` ever reads: on
Linux it is parsed, validated, and silently ignored, while the matrix reports
scroll injection ✅ on all three backends, because scroll injection does in fact
work. Hints are ✅ while `DrawHintSearchInput` in the Linux overlay manager
refuses unconditionally on every backend. A subsystem can be green in every cell
while a word a person wrote in their config file means nothing.

So we decided the unit of measurement rather than the backlog: **parity is a
promise about every name a person can write** — every Option, mode flag, action
and command — and it is a *behavioral* promise on one named Linux stack,
**wlroots Wayland with a CGO build**. That names a protocol family — layer
shell, `zwlr_virtual_pointer`, evdev — rather than one compositor: sway,
Hyprland, niri, River and Wayfire are all in it, sway is what CI runs, and a
single compositor's upstream defect is a documented exception rather than a
parity failure. niri is the live example: its tiled windows expose no on-screen
position (niri#2381), so hints misalign there, and no amount of work in this
repository changes that. The other supported backends owe the same
capabilities and carry their own documented limits. Three capabilities are
exempt because the concept is macOS-only, and the list is closed: secure input
detection, screen-sharing hide, and system cursor hide. Anything not on that
list is a gap, whatever the matrix currently claims.

This is the `Vocabulary` entry in `CONTEXT.md` applied to platforms. A
vocabulary is "a closed set of names a person can write and Neru promises to
recognise"; a name that is recognised on one platform and inert on another is
that promise broken, and the matrix is structurally unable to see it.

## Considered options

- **Matrix equality — every Linux cell matches macOS.** Cheap to state and
  cheap to check, and it is what the repo has been doing. It is how
  `smooth_scroll` reached the config schema, the documentation and the
  validators without anyone noticing that no Linux code path reads it. A
  measure that cannot detect its own most recent failure is not a measure.
- **Behavioral equivalence on all three backends.** The honest reading of "make
  Linux match macOS", and unachievable. X11 cannot do modifier passthrough at
  all: `XGrabKeyboard` is all-or-nothing, and `XSendEvent` is ignored by most
  applications. Adopting this definition would mean either never reaching
  parity or deleting a working Wayland feature to level down.
- **Blessing X11 instead of wlroots.** X11 is the more widely deployed, has
  proper global hotkeys through `XGrabKey` rather than a passive evdev read, and
  needs no `input`-group membership; AT-SPI reports screen coordinates there, so
  it escapes the window-origin problem that Wayland clients cannot solve for
  themselves. It was rejected for its ceiling. Its two deltas — modifier
  passthrough, and an unmodified scroll carrying whatever modifiers the X server
  records the user as physically holding — are properties of the display server,
  and the first has no fix at any budget. Blessing the stack whose gaps cannot
  close makes the goal unreachable by construction. wlroots' remaining gaps are
  operational: an `input`-group setup step, and window origins resolved through
  compositor IPC.
- **Keeping the matrix as the definition and adding a second list for words.**
  Two lists, one of which is a superset of the other, and no rule saying which
  wins. `docs/CROSS_PLATFORM.md` already owns capability status; the fix is to
  make its gap list answer the vocabulary question, not to grow a rival.

## Consequences

- **Platform support per word is declared once and guardrailed.** It lives
  beside the config schema and the modecmd descriptor table — the two places
  that already own the words — and the load-time warning, `neru doctor` and the
  documentation rows are projections of it, the way ADR 0006 settled for
  Options and ADR 0008 for vocabularies generally. The guardrail is the ADR 0011
  case exactly: an option that is neither shared nor declared compiles, lints,
  passes every test, and ships as a silent no-op, which is precisely how
  `smooth_scroll` got in. The declaration carries a **platform column, not a
  darwin-only flag**, so Windows rows exist from the first commit even though no
  Windows work is scheduled here — Windows has ten known gaps of its own and the
  same disease, and retrofitting a column into a boolean later would mean
  touching every call site.
- **A darwin-only option warns; it never errors.** ADR 0008 keeps `Insert` and
  `F21`–`F24` in the shared key vocabulary although macOS has no Carbon keycode
  for them, so that one config file works on every platform. Refusing to start
  on an option that is inert here would trade a silent lie for a portability
  break. The daemon says so once at load, `neru doctor` carries a row, and it
  runs.
- **Two "exclusives" were not exclusive.** Smooth scroll is documented as
  needing "a synthesizable continuous scroll event stream"; uinput has
  `REL_WHEEL_HI_RES`, `wl_pointer` axis values are continuous, and libei carries
  scroll deltas, so it moves to the gap list pending a spike that could still
  kill it. *(Amended: the spike ran and smooth scroll ships. This paragraph's
  claim that all three injection paths have the primitive was two thirds right —
  see the amendment at the end.)* The Vision (OCR) hint strategy is macOS-only for want of an engine,
  not for want of an API, and Linux arguably needs it more than macOS does —
  AT-SPI coverage is toolkit-dependent in a way the AX tree is not. It moves to
  the gap list too. System cursor hide stays exempt, but its stated reason was
  wrong: `xfixes` is already linked in `platform/linux/cgo.go`, so X11 could do
  it. Wayland cannot — a client may not hide another client's cursor — and the
  blessed stack is Wayland.
- **The OCR engine is linked, like every other native dependency.** Released
  Linux binaries are already built `CGO_ENABLED=1` and dynamically linked, and
  step 1 of `LINUX_SETUP.md` already makes twelve shared libraries a required
  install — so `#cgo pkg-config: tesseract lept` costs one more line on a list
  that exists rather than a new kind of obligation. It buys the idiom the whole
  Linux tree uses, the `_cgo`/`_nocgo` twin for free, in-process operation, and
  a C API that yields word-level bounding boxes directly instead of parsed
  output. The alternatives were a `PATH`-discovered binary and `dlopen`; the
  first breaks the idiom for one feature, the second invents a symbol-binding
  pattern nothing else in the tree pays for, and turns ABI drift into a runtime
  hazard.
- **Which does not make the engine's absence a startup failure to shrug at.**
  Tesseract is the first library on that list required for a *non-default*
  strategy — `hints.strategy` defaults to `axtree`, so most installs will never
  invoke it, and under dynamic linking a missing `libtesseract.so` stops the
  daemon before any Neru code runs. That is ADR 0012's criterion, so the
  install documentation names it as required rather than optional, and the
  packaging lists carry it.
- **Runtime resolution survives the linking decision anyway.** An OCR engine
  needs language data at use — `tessdata`, found through `TESSDATA_PREFIX` and
  shipped as its own distribution package. No linking strategy resolves that,
  so the "fails loudly when it cannot resolve" behavior exists regardless, in
  the one place it genuinely must: a `Health` check that reports
  `CodeNotSupported` naming the missing language data.
- **The `vision` strategy is not one capability, and only part of it ports.**
  `VisionPort.DetectElements` runs three macOS requests — text recognition,
  rectangle detection and saliency — and `config.HintsVisionConfig` exposes the
  second as `detect_rectangles` plus four `rectangle_*` tuning options. An OCR
  engine answers the text half only, so those five options are **declared
  darwin-only now** rather than left undefined until an OCR engine lands. The
  alternative was contour-based rectangle detection, which means a heavy new
  required library for a sub-feature of a strategy that is not the default —
  failing the "every option is a cost" rule twice over. Linux `vision` is
  text-only and says so. Deferring the decision was the worst of the three
  options available, because it leaves five options in exactly the undefined
  state this ADR exists to end.
- **OCR needs a screen-capture backend that does not exist.** There is no
  capture code anywhere in the tree — no screencopy, no PipeWire, no
  `XGetImage` — and `adapter/vision/adapter_other.go` stubs `CaptureScreen`
  alongside `DetectElements`. Capture is therefore a prerequisite in its own
  right, taken per backend (`wlr-screencopy` on wlroots, `XGetImage` on X11,
  the portal only for KDE) rather than through xdg-desktop-portal ScreenCast
  everywhere, because a consent picker in front of a hint refresh is a latency
  and consent-fatigue regression the blessed stack has no need to pay.
- **The non-blessed backends get bounded fixes, not behavioral parity.** X11's
  unmodified-scroll leak has a real symptom — binding `Ctrl+J` to a plain
  `scroll_down` sends ctrl+scroll, which most applications read as zoom, for as
  long as ctrl is held — and a known fix in reading live key state through
  `XQueryKeymap` in the C bridge. It gets fixed. X11 modifier passthrough is
  documented as display-server-inherent and closed.
- **Linux behavior becomes observable.** The definition is behavioral and the
  project is developed on macOS, where nothing can run it; CI runs `just
  test-ci` on `ubuntu-latest` with no display server, so every Linux test that
  would touch one skips, and `test-desktop` runs on no platform at all. A
  headless sway job is what makes any claim in this ADR checkable — and a
  disposable runner is the one place the desktop-driving tier is safe to run,
  since the reason it is opt-in locally is that it commandeers the machine.
- **Four things are outside the boundary, and say so.** GNOME/Mutter Wayland
  and Cosmic are supported-desktop questions, not parity gaps. The
  `CGO_ENABLED=0` Linux build is a near-total stub — full `CodeNotSupported`
  mirrors across cursor, click, scroll, hotkeys, overlay, screens and focus —
  for a configuration macOS does not offer at all; it is a distribution
  convenience and announces that at startup rather than failing feature by
  feature. Service management covers systemd user units; other init systems
  report `CodeNotSupported`.
- **A feature is not complete until its platform support is decided.** Closing
  fourteen gaps changes nothing if the fifteenth is created next month, and
  nothing that existed before this ADR would have stopped it — `smooth_scroll`
  reached the schema, the validators and the documentation without a Linux code
  path and broke no rule. macOS may still ship first; it is the reference
  implementation and the platform the maintainer runs. What changes is that a
  feature landing on macOS alone is *incomplete* until Linux has it or it is
  declared, and that decision is made at merge, as a Known Gaps entry, rather
  than discovered by an audit two years later. For config options this is
  enforced rather than promised — the guardrail above already fails the build.
  The policy covers what the guardrail cannot see: commands, actions, and
  behavior.
- **Linux graduates on a stated criterion.** `CROSS_PLATFORM.md` defines
  **Stable** as "fully featured — a gap on this platform is a bug", which is
  nearly a restatement of parity, and nothing said when the label flips. It
  flips when the Linux entries in Known Gaps are empty for the blessed stack
  and the sway CI job is green and required for merge. A label with no
  criterion either never moves or moves on a feeling; fourteen tickets with no
  defined end state is how "beta" becomes permanent.
- **Two new words in `CONTEXT.md`.** *Parity* and *Blessed stack*. The first
  because the word is the decision and it was previously used loosely enough to
  mean either measure; the second because without it *Parity* has to re-explain
  which Linux it is talking about every time it is used.

## Amendment: X11 has no sub-notch scroll

The smooth-scroll spike this ADR called for has run, and one sentence above
needs correcting. "uinput has `REL_WHEEL_HI_RES`, `wl_pointer` axis values are
continuous, and libei carries scroll deltas" was offered as evidence that all
three Linux injection paths have the primitive. Two of them do. **X11 does
not**, and no budget changes that.

Core X11 scrolling is buttons 4 to 7, and a button event is one notch by
definition — the XI2 protocol specification says so in as many words: "One unit
of scrolling in either direction is considered to be equivalent to one button
event". The smooth-scroll path real devices use is an XI2 scroll valuator, and
the XTEST pointer that `XTestFakeButtonEvent` drives has none: the X server
allocates it through `CorePointerProc` (`dix/devices.c`) with exactly two axes,
`Rel X` and `Rel Y`, and no XI2 request lets a client add a scroll class to a
device — only an input driver can, at device init. So on X11 there is no
sub-notch value to send, and this is a property of the display server, alongside
modifier passthrough, rather than unbuilt work.

That does not make smooth scroll an exclusive again, and the option is not
declared inert on Linux. What a person writes `smooth_scroll` for is a scroll
that arrives as a movement instead of a jump, and X11 delivers that from two
notches up: the animator spreads the same eased curve over whole notches,
honouring `steps`, `max_duration` and `duration_per_pixel` exactly as the other
backends do, and travelling exactly as far.

Be precise about the size where it does not, because it is the common one. A
scroll worth one notch has one event to send however it is scheduled, and the
default `scroll_step` of 50 pixels is exactly one notch, so a plain
`scroll_down` on X11 is not animated — it goes out immediately and unchanged,
rather than being held back to arrive on a curve it cannot express. Turning the
option on there buys nothing for that binding and costs nothing either, and
`scroll_step_half` and `scroll_step_full` do animate. Granularity is the shape
this ADR already has a slot for — "the other supported backends owe the same
capabilities and carry their own documented limits" — and the limit is
documented as footnote ⁴ of the Capability Matrix, in the size a person will
actually notice rather than as a protocol fact.

Two further findings from the spike, recorded because they cost the next person
the same day otherwise:

- **The wlroots path that carries the fraction is the virtual pointer, not
  uinput.** `zwlr_virtual_pointer_v1.axis` leaves the discrete step count at
  zero, and wlroots forwards a zero step count to the focused client as a plain
  continuous `wl_pointer.axis` with no accumulator in front of it. The uinput
  `REL_WHEEL_HI_RES` route arrives as `axis_value120` instead, which a client
  older than `wl_pointer` version 8 accumulates to whole notches before it sees
  anything. The animator uses the virtual pointer for that reason, and because
  it is the path the headless-sway job can actually observe: that job runs
  `WLR_BACKENDS=headless` with no libinput, so nothing written to a uinput
  device reaches the compositor there at all.
- **The claim is measured on wlroots only, and on one path of it.**
  `TestScrollAtCursor_DeliversSubNotchStepsWithSmoothScroll` maps a real
  xdg-shell window under headless sway and asserts that a sub-notch delta
  arrives with no notch count attached and with axis source `continuous`. Three
  things are *not* measured and say so wherever they are stated: the X11
  conclusion, the KWin one (read from libei's `ei_device_scroll_delta`
  contract), and the uinput `REL_WHEEL_HI_RES` route, which that job cannot
  observe at all. An Xvfb harness would let the X11 half be checked rather than
  argued.
