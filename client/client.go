// Package client talks to a running Neru daemon over its IPC endpoint, using
// the protocol in https://github.com/y3owk1n/neru/blob/main/docs/reference/ipc.md.
//
// Status, the queries, Watch and Ping return typed replies. Every other
// command goes through Send, which takes the words `neru` takes after the
// command name.
//
// The client speaks protocol 1, so it needs a daemon whose replies carry a
// protocol number. Like the protocol, this package breaks only at a major Neru
// release.
package client

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net"
	"syscall"

	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/ipcendpoint"
)

// ProtocolVersion is the protocol this package speaks. Every request carries
// it, and a daemon on a lower one refuses the request with
// ERR_VERSION_MISMATCH.
const ProtocolVersion = 1

// ErrNoDaemon is what a call returns when no daemon listens on the endpoint.
// A daemon that is up while Neru is stopped answers instead, refusing most
// commands with ERR_NOT_RUNNING.
var ErrNoDaemon = errors.New("no neru daemon is listening")

// Error is a request the daemon refused. Code is one of the ERR_ codes the
// IPC reference lists under Response codes.
type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string {
	return e.Code + ": " + e.Message
}

// Client sends requests to this user's daemon. Each call opens its own
// connection, so a Client is safe to share.
type Client struct{}

// New returns a client for this user's daemon. Each call finds the endpoint
// the way the neru CLI does, so a client made before the daemon starts still
// reaches it.
func New() *Client {
	return &Client{}
}

type request struct {
	Protocol int      `json:"protocol"`
	Action   string   `json:"action"`
	Args     []string `json:"args,omitempty"`
}

type reply struct {
	Protocol int             `json:"protocol"`
	Success  bool            `json:"success"`
	Message  string          `json:"message"`
	Code     string          `json:"code"`
	Data     json.RawMessage `json:"data"`
}

// Send runs one command with args and returns the data of its reply, which is
// empty for a command that answers with none.
func (c *Client) Send(ctx context.Context, action string, args ...string) (json.RawMessage, error) {
	answer, err := c.exchange(ctx, action, args, nil)
	if err != nil {
		return nil, err
	}

	return answer.Data, nil
}

// Ping returns the protocol the daemon speaks. A number above ProtocolVersion
// means the daemon may have changed something this package relies on.
func (c *Client) Ping(ctx context.Context) (int, error) {
	answer, err := c.exchange(ctx, "ping", nil, nil)
	if err != nil {
		return 0, err
	}

	return answer.Protocol, nil
}

// Status returns what `neru status --json` prints.
func (c *Client) Status(ctx context.Context) (StatusData, error) {
	return call[StatusData](ctx, c, "status")
}

// Displays returns every connected display.
func (c *Client) Displays(ctx context.Context) (DisplaysData, error) {
	return call[DisplaysData](ctx, c, "query-displays")
}

// Cursor returns where the cursor is and which display holds it.
func (c *Client) Cursor(ctx context.Context) (CursorData, error) {
	return call[CursorData](ctx, c, "query-cursor")
}

// Window returns the focused window's bounds, or nil when no window has focus.
func (c *Client) Window(ctx context.Context) (*WindowData, error) {
	return call[*WindowData](ctx, c, "query-window")
}

// App returns the focused application.
func (c *Client) App(ctx context.Context) (AppData, error) {
	return call[AppData](ctx, c, "query-app")
}

// Hints returns the elements hints mode would label. flags are the
// collection flags `neru query hints` takes, such as "--role", "button".
func (c *Client) Hints(ctx context.Context, flags ...string) (HintsData, error) {
	return call[HintsData](ctx, c, "query-hints", flags...)
}

// Watch calls each with every line of the watch stream: the status snapshot
// it opens with, then one line per event. It returns nil when the daemon ends the
// stream, the error each returns, or the context's error once it is done.
func (c *Client) Watch(ctx context.Context, each func(WatchLine) error) error {
	_, err := c.exchange(ctx, "watch", nil, func(decoder *json.Decoder) error {
		for {
			var line WatchLine

			decodeErr := decoder.Decode(&line)
			if errors.Is(decodeErr, io.EOF) {
				return nil
			}

			if decodeErr != nil {
				return derrors.Wrap(decodeErr, derrors.CodeIPCFailed, "read the watch stream")
			}

			eachErr := each(line)
			if eachErr != nil {
				return eachErr
			}
		}
	})

	return err
}

// call runs a command whose reply data decodes into T.
func call[T any](ctx context.Context, c *Client, action string, args ...string) (T, error) {
	var data T

	raw, err := c.Send(ctx, action, args...)
	if err != nil {
		return data, err
	}

	// A reply with no data, such as query-window with no window focused,
	// leaves the zero value.
	if len(raw) == 0 {
		return data, nil
	}

	err = json.Unmarshal(raw, &data)
	if err != nil {
		return data, derrors.Wrapf(
			err,
			derrors.CodeSerializationFailed,
			"read the %s reply",
			action,
		)
	}

	return data, nil
}

// exchange sends one request and reads its reply. A refused request returns
// *Error. When stream is set and the request succeeded, stream reads the rest
// of the connection.
//
// Canceling ctx closes the connection, which is what ends a call blocked on
// a read, a watch stream included.
func (c *Client) exchange(
	ctx context.Context,
	action string,
	args []string,
	stream func(*json.Decoder) error,
) (reply, error) {
	endpoint := ipcendpoint.ClientPath()

	conn, err := ipcendpoint.Dial(ctx, net.Dialer{}, endpoint)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ECONNREFUSED) {
			err = ErrNoDaemon
		}

		return reply{}, derrors.Wrapf(err, derrors.CodeIPCFailed, "dial %s", endpoint)
	}
	defer func() { _ = conn.Close() }()

	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()

	answer, err := talk(
		conn,
		request{Protocol: ProtocolVersion, Action: action, Args: args},
		stream,
	)

	// A read on a connection the context closed fails with a closed-connection
	// error, and the context's own error is the one worth reporting.
	if err != nil && ctx.Err() != nil {
		return answer, ctx.Err()
	}

	return answer, err
}

// talk writes req to conn and reads the reply, then hands the rest of the
// connection to stream when it is set and the request succeeded.
func talk(conn net.Conn, req request, stream func(*json.Decoder) error) (reply, error) {
	err := json.NewEncoder(conn).Encode(req)
	if err != nil {
		return reply{}, derrors.Wrapf(err, derrors.CodeIPCFailed, "send %s", req.Action)
	}

	decoder := json.NewDecoder(conn)

	var answer reply

	err = decoder.Decode(&answer)
	if err != nil {
		return reply{}, derrors.Wrapf(err, derrors.CodeIPCFailed, "read the %s reply", req.Action)
	}

	if !answer.Success {
		return answer, &Error{Code: answer.Code, Message: answer.Message}
	}

	if stream != nil {
		return answer, stream(decoder)
	}

	return answer, nil
}
