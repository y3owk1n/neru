# Build on Neru

How to build tools on top of Neru, such as a status bar, a launcher, voice
control or another program that drives the cursor. Every command a hotkey
runs is also a `neru` command and an IPC request, so a tool can do anything a
binding can, read what Neru sees, and follow what it does. Shell recipes are
in [Scripting](scripting.md). With a coding agent, the
`neru-build-integration` skill makes the same choices, checked against your
installed version. See [Set up with an agent](installation.md#set-up-with-an-agent).

## Pick an interface

| What you want                              | What to use                                         | Where it is documented |
| :----------------------------------------- | :-------------------------------------------------- | :--------------------- |
| React to events from your config           | `[hooks]`                                           | [Configuration reference](../reference/configuration.md#hooks) |
| Run commands from a script                 | The `neru` CLI                                      | [CLI reference](../reference/cli.md) |
| Read state as JSON                         | `--json` on `status`, `query`, `doctor`, `roles` and `services status` | [CLI reference](../reference/cli.md#neru-status) |
| Follow state live                          | `neru watch`, narrowed with `--event`               | [`neru watch`](../reference/cli.md#neru-watch) |
| Find elements and their hint labels        | `neru query hints`                                  | [`neru query`](../reference/cli.md#neru-query) |
| Supply your own targets to hints mode      | `neru hints --targets`                              | [Supplying your own targets](../reference/cli.md#supplying-your-own-targets) |
| Talk to the socket directly                | JSON over the IPC endpoint                          | [IPC protocol](../reference/ipc.md) |
| Drive Neru from a Go program               | The `client` package                                | [Go client](#go-client) |
| Validate configs or IPC payloads           | `schemas/config.schema.json`, `schemas/ipc.schema.json` | [Editor support](../reference/configuration.md#editor-support), [Schemas](../reference/ipc.md#schemas) |

## Use cases

### A status bar that shows the mode

A bar that runs one long-lived command can read `neru watch` instead of
starting a process per hook. `--event` drops the events the bar does not
need. The stream still opens with a snapshot of the whole state.

```bash
neru watch --event mode_enter,mode_exit
```

The full script, with the `jq` filter and an Eww example, is in
[Keep a status bar in sync](scripting.md#keep-a-status-bar-in-sync).

### A health check

[`neru doctor --json`](../reference/cli.md#neru-doctor) prints every check as
one object, so a script can test the exit status and read the details from
the object. The reference says what each key means.

```bash
neru doctor --json | jq '{healthy, config_error: .config.error, reachable: .ipc_endpoint.reachable}'
```

### A launcher that shows Neru's state

A launcher command such as a Raycast script can read `neru status --json`
and show the mode, and whether Neru is paused. The command fails when no
daemon is running.

```bash
neru status --json | jq -r '"\(.mode), \(if .enabled then "running" else "paused" end)"'
```

### Voice control or a companion UI

`neru query hints --json` lists the elements hints mode would label, with the
label each one would get. A voice tool can read the titles out, and a
companion UI can draw its own view of them. To select one, open hints mode
with the same flags, then type the label into it with `neru action feed
--mode`:

```bash
label=$(neru query hints --text Save --json | jq -r '.hints[0].label')
neru hints --text Save --action left_click
neru action feed --mode $(echo "$label" | fold -w1)
```

A label holds only while the screen does not change, and only for the same
flags. The rules are under [`neru query`](../reference/cli.md#neru-query).

### A tool that knows where the controls are

When no hint strategy finds a control, a tool that knows its bounds can hand
them to hints mode. `--targets -` reads the JSON from stdin, in the shape
`neru query hints --json` prints. Each target needs `x`, `y`, `width` and
`height`.

```bash
echo '{"hints":[{"x":100,"y":38,"width":44,"height":52,"title":"Back"}]}' \
  | neru hints --targets - --action left_click
```

What `--targets` refuses and how its targets behave is under
[Supplying your own targets](../reference/cli.md#supplying-your-own-targets).

## Go client

A Go program can import `github.com/y3owk1n/neru/client` instead of writing
the protocol itself. It finds the endpoint the way the `neru` CLI does, and
returns typed replies for the status, the queries and the watch stream.
Every other command goes through `Send`, which takes the words `neru` takes
after the command name.

```bash
go get github.com/y3owk1n/neru/client
```

It needs Go 1.26.5, the version Neru's `go.mod` declares, and a daemon from
the release that added the package or later. An older daemon refuses its
requests.

```go
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/y3owk1n/neru/client"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	neru := client.New()

	status, err := neru.Status(ctx)
	if errors.Is(err, client.ErrNoDaemon) {
		log.Fatal("neru is not running, start it with neru launch")
	}
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("mode:", status.Mode)

	// Commands without a method of their own go through Send.
	_, err = neru.Send(ctx, "hints", "--action", "left_click")

	var refusal *client.Error
	if errors.As(err, &refusal) {
		log.Fatalf("neru refused: %s", refusal.Code) // such as ERR_MODE_DISABLED
	}
	if err != nil {
		log.Fatal(err)
	}
}
```

- `ErrNoDaemon` means nothing listens on the endpoint.
- `*client.Error` is a request the daemon refused. Its `Code` is one of the
  [response codes](../reference/ipc.md#response-codes).

`Watch` calls your function with each line of the stream: the snapshot it
opens with, then one line per event. Name events to receive only those. It
returns nil when the daemon exits, and the context's error once the context
is done.

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/y3owk1n/neru/client"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// Print the mode now, then each time it changes.
	err := client.New().Watch(ctx, func(line client.WatchLine) error {
		switch {
		case line.Event == "snapshot":
			fmt.Println(line.Status.Mode)
		case line.Event == "mode_enter":
			fmt.Println(line.Mode)
		case line.Event == "mode_exit" && line.Reason != "switched":
			fmt.Println("idle")
		}

		return nil
	}, "mode_enter", "mode_exit")
	if err != nil && ctx.Err() == nil {
		log.Fatal(err)
	}
}
```

The package follows the protocol's
[compatibility rule](../reference/ipc.md#compatibility). It breaks only at a
major Neru release, so ignore fields and events you do not know. The full API
is on [pkg.go.dev](https://pkg.go.dev/github.com/y3owk1n/neru/client).
