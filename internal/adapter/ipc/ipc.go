package ipc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/derrors"
)

const (
	// SocketName is the name of the Unix socket file.
	SocketName = "neru.sock"

	// DefaultTimeout is the default timeout for IPC operations.
	DefaultTimeout = 5 * time.Second

	// ConnectionTimeout is the timeout for establishing a connection.
	ConnectionTimeout = 2 * time.Second

	// ConnectionReadTimeout is the timeout for reading from a connection.
	ConnectionReadTimeout = 30 * time.Second

	// ConnectionWriteTimeout bounds how long the server waits to hand a
	// response to a client that has stopped reading. It is applied after the
	// handler returns, so a long-running command does not spend the budget the
	// reply needs.
	ConnectionWriteTimeout = 5 * time.Second

	// PingTimeout is the timeout for ping operations.
	PingTimeout = 500 * time.Millisecond

	// listenerCloseTimeout bounds how long Stop waits for the listener to
	// close. See closeListener for why the close can fail to return at all.
	listenerCloseTimeout = 2 * time.Second

	// DefaultSocketPerms is the default socket permissions.
	DefaultSocketPerms = 0o600

	// maxCommandBytes bounds one command. A command is a verb, a small
	// parameter map and its arguments; the largest realistic one — a config
	// value or a sequence definition traveling in Args — is orders of
	// magnitude below this. The cap exists so a peer cannot make the daemon
	// buffer without limit, not to police command shape, which
	// DisallowUnknownFields already does.
	maxCommandBytes = 64 << 10

	// defaultBuildVersion is the fallback version when SetBuildVersion is not called.
	defaultBuildVersion = "dev"
)

// buildVersion holds the application build version, set at startup via SetBuildVersion.
// Both the client and server use this to detect CLI/daemon version mismatches.
// Access is guarded by atomic.Value so concurrent reads from IPC goroutines are safe.
var buildVersion atomic.Value //nolint:gochecknoglobals
func init() {
	buildVersion.Store(defaultBuildVersion)
}

// SetBuildVersion sets the build version used for IPC version validation.
// Call this early in program startup (e.g. from cli.init) before any IPC
// operations. Both the CLI client and the daemon must call this so the
// version embedded in commands matches the version the server expects.
func SetBuildVersion(v string) {
	if v != "" {
		buildVersion.Store(v)
	}
}

// BuildVersion returns the current build version used for IPC validation.
func BuildVersion() string {
	v, ok := buildVersion.Load().(string)
	if !ok {
		return defaultBuildVersion
	}

	return v
}

// Standard response codes used to indicate the result of IPC operations.
const (
	CodeOK              = "OK"
	CodeUnknownCommand  = "ERR_UNKNOWN_COMMAND"
	CodeNotRunning      = "ERR_NOT_RUNNING"
	CodeAlreadyRunning  = "ERR_ALREADY_RUNNING"
	CodeModeDisabled    = "ERR_MODE_DISABLED"
	CodeInvalidInput    = "ERR_INVALID_INPUT"
	CodeActionFailed    = "ERR_ACTION_FAILED"
	CodeVersionMismatch = "ERR_VERSION_MISMATCH"

	// CodeChainBail indicates a chain action should abort (e.g. user canceled
	// a mode without making a selection).
	CodeChainBail = "ERR_CHAIN_BAIL"

	// CodeNotSupported indicates the operation is not supported on this platform.
	CodeNotSupported = "ERR_NOT_SUPPORTED"
)

// Command is a command sent through the IPC interface.
type Command struct {
	Version string         `json:"version,omitempty"`
	Action  string         `json:"action"`
	Params  map[string]any `json:"params,omitempty"`
	Args    []string       `json:"args,omitempty"`
}

// Response is a response returned through the IPC interface.
type Response struct {
	Version string `json:"version,omitempty"`
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Code    string `json:"code,omitempty"`
	Data    any    `json:"data,omitempty"`
}

// StatusData is the payload structure for status query responses.
type StatusData struct {
	Enabled bool   `json:"enabled"`
	Mode    string `json:"mode"`
	Config  string `json:"config"`
}

// Server handles incoming IPC connections and routes commands to handlers.
type Server struct {
	listener   net.Listener
	logger     *zap.Logger
	handler    CommandHandler
	socketPath string
	wg         sync.WaitGroup
}

// CommandHandler is the interface for processing IPC commands.
type CommandHandler func(ctx context.Context, cmd Command) Response

// SocketPath returns the platform IPC endpoint path (Unix socket or named pipe)
// a client should use.
//
// It is the endpoint the daemon listens on in every ordinary case. Where the
// transport can tell that a live daemon answers somewhere else — an endpoint
// left by a daemon started before this version, or one in a runtime directory
// this process's environment does not name — it returns that instead, so a CLI
// reaches the daemon that is actually running rather than reporting none.
func SocketPath() string {
	return clientEndpointPath()
}

// errCommandTooLarge is returned by boundedReader once a command has spent its
// budget. It is a sentinel so the decode failure can be reported as the size
// refusal it is rather than as the truncated JSON it looks like.
var errCommandTooLarge = errors.New("command exceeds the maximum size")

// boundedReader is io.LimitReader with a distinguishable ending: hitting the
// limit is an error rather than a clean EOF.
type boundedReader struct {
	reader    io.Reader
	remaining int64
}

func (b *boundedReader) Read(buf []byte) (int, error) {
	if b.remaining <= 0 {
		return 0, errCommandTooLarge
	}

	if int64(len(buf)) > b.remaining {
		buf = buf[:b.remaining]
	}

	read, err := b.reader.Read(buf)
	b.remaining -= int64(read)

	return read, err
}

// NewServer creates a new IPC server instance with the specified handler.
func NewServer(handler CommandHandler, logger *zap.Logger) (*Server, error) {
	if logger == nil {
		logger = zap.NewNop()
	}

	logger = logger.Named("ipc")
	// The daemon binds the transport's preferred endpoint, never the one
	// SocketPath may have resolved to some other running daemon's.
	socketPath := daemonEndpointPath()

	listener, listenerErr := listenEndpoint(context.Background(), socketPath)
	if listenerErr != nil {
		return nil, derrors.Wrap(
			listenerErr,
			derrors.CodeIPCFailed,
			"failed to create IPC endpoint",
		)
	}

	logger.Info("IPC server created", zap.String("endpoint", socketPath))

	return &Server{
		listener:   listener,
		logger:     logger,
		handler:    handler,
		socketPath: socketPath,
	}, nil
}

// Start begins accepting connections on the IPC server.
func (s *Server) Start() {
	go func() {
		for {
			connection, connectionErr := s.listener.Accept()
			if connectionErr != nil {
				// If listener is closed, exit gracefully
				if errors.Is(connectionErr, net.ErrClosed) {
					s.logger.Debug("IPC server listener closed, stopping accept loop")

					return
				}

				s.logger.Error("Failed to accept connection", zap.Error(connectionErr))

				continue
			}

			s.wg.Add(1)

			go s.handleConnection(connection)
		}
	}()
}

// Stop terminates the IPC server and cleans up resources.
func (s *Server) Stop() error {
	closeListenerErr := s.closeListener()
	if closeListenerErr != nil {
		return derrors.Wrap(closeListenerErr, derrors.CodeIPCFailed, "failed to close listener")
	}

	done := make(chan struct{})

	go func() {
		s.wg.Wait()
		close(done)
	}()

	// Use timer instead of time.After to prevent memory leaks
	timer := time.NewTimer(1 * time.Second)
	defer timer.Stop()

	select {
	case <-done:
		// All connections closed successfully
	case <-timer.C:
		s.logger.Warn("IPC server: timeout waiting for connections to close")
	}

	cleanupErr := cleanupEndpoint(s.socketPath)
	if cleanupErr != nil {
		return derrors.Wrap(cleanupErr, derrors.CodeIPCFailed, "failed to clean up IPC endpoint")
	}

	return nil
}

// closeListener closes the listener, giving up if the close never returns.
// The Windows named-pipe listener can hang: an aborting accept can consume the
// single close signal, and if the client had already disconnected the listener
// retries and waits for a second signal nobody sends. Every `neru status`
// probe connects-and-drops, so a shutdown racing one reaches this. Abandoning
// costs a goroutine the exiting process reclaims anyway; waiting costs the
// exit itself.
func (s *Server) closeListener() error {
	if s.listener == nil {
		return nil
	}

	closed := make(chan error, 1)

	go func() {
		closed <- s.listener.Close()
	}()

	timer := time.NewTimer(listenerCloseTimeout)
	defer timer.Stop()

	select {
	case closeErr := <-closed:
		return closeErr
	case <-timer.C:
		s.logger.Warn("Timed out closing the IPC listener; abandoning it",
			zap.Duration("waited", listenerCloseTimeout))

		return nil
	}
}

// handleConnection processes a single client connection and executes the received command.
func (s *Server) handleConnection(connection net.Conn) {
	traceID := NewTraceID()
	logger := s.logger.With(zap.String("trace_id", traceID.String()))

	// Create context with trace ID
	ctx := WithTraceID(context.Background(), traceID)

	defer func() {
		connectionCloseErr := connection.Close()
		if connectionCloseErr != nil {
			logger.Error("Failed to close connection", zap.Error(connectionCloseErr))
		}

		s.wg.Done()
	}()

	// Who is on the other end is settled before anything they sent is read.
	// A connection that fails this gets no reply at all: there is nothing
	// useful to tell a caller that should not have reached the daemon, and a
	// reply would confirm the endpoint to whoever found it.
	authErr := authorizePeer(connection)
	if authErr != nil {
		logger.Warn("Refused an IPC connection from another user", zap.Error(authErr))

		return
	}

	// Only the read side is bounded here: this guards a client that connects
	// and never sends a command. The write side gets its own deadline once the
	// handler has finished, in writeResponse.
	deadlineErr := connection.SetReadDeadline(time.Now().Add(ConnectionReadTimeout))
	if deadlineErr != nil {
		logger.Error("Failed to set connection read deadline", zap.Error(deadlineErr))

		return
	}

	decoder := json.NewDecoder(&boundedReader{reader: connection, remaining: maxCommandBytes})
	decoder.DisallowUnknownFields()

	encoder := json.NewEncoder(connection)

	// reply sends one response and reports the outcome. A client that has
	// already gone away is the expected end of a command that outlived its
	// caller's patience, so it is logged quietly; anything else is a fault.
	reply := func(response Response) {
		writeErr := writeResponse(connection, encoder, response)
		switch {
		case writeErr == nil:
		case isPeerGoneErr(writeErr):
			logger.Debug("Client disconnected before response was sent", zap.Error(writeErr))
		default:
			logger.Error("Failed to encode response", zap.Error(writeErr))
		}
	}

	var cmd Command

	decodeCommandErr := decoder.Decode(&cmd)
	if decodeCommandErr != nil {
		if errors.Is(decodeCommandErr, errCommandTooLarge) {
			logger.Error("Refused an oversized command", zap.Int("limit_bytes", maxCommandBytes))

			reply(Response{
				Success: false,
				Message: fmt.Sprintf("command exceeds the %d byte limit", maxCommandBytes),
				Code:    CodeInvalidInput,
			})

			return
		}

		logger.Error("Failed to decode command", zap.Error(decodeCommandErr))

		reply(Response{
			Success: false,
			Message: fmt.Sprintf("failed to decode command: %v", decodeCommandErr),
			Code:    CodeInvalidInput,
		})

		return
	}

	logger.Debug(
		"Received command",
		zap.String("action", cmd.Action),
		zap.String("version", cmd.Version),
	)

	// Validate build version if provided
	serverVersion := BuildVersion()
	if cmd.Version != "" && cmd.Version != serverVersion {
		logger.Warn("Build version mismatch",
			zap.String("client_version", cmd.Version),
			zap.String("server_version", serverVersion))

		reply(Response{
			Version: serverVersion,
			Success: false,
			Message: fmt.Sprintf(
				"version mismatch: client=%s, server=%s — please restart the neru daemon",
				cmd.Version,
				serverVersion,
			),
			Code: CodeVersionMismatch,
		})

		return
	}

	response := s.handler(ctx, cmd)
	// Always include server version in response
	response.Version = serverVersion

	reply(response)
}

// writeResponse sends one response to the client.
//
// The deadline set when the connection was accepted budgets the whole exchange,
// including however long the handler ran. Handlers are allowed to take longer
// than that — an action sequence can sleep, or wait for the user to finish a
// mode — so the reply is given its own window here. Without it, a slow but
// perfectly successful command would have its reply refused before it was even
// attempted, and the caller would see a timeout for work that did happen.
func writeResponse(connection net.Conn, encoder *json.Encoder, response Response) error {
	deadlineErr := connection.SetWriteDeadline(time.Now().Add(ConnectionWriteTimeout))
	if deadlineErr != nil {
		return deadlineErr
	}

	return encoder.Encode(response)
}

// isPeerGoneErr reports whether err is the expected result of writing to a
// connection whose peer has already gone away (broken pipe, connection reset,
// or an already-closed pipe/connection). These are benign races, not faults.
func isPeerGoneErr(err error) bool {
	return errors.Is(err, syscall.EPIPE) ||
		errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, net.ErrClosed) ||
		errors.Is(err, io.ErrClosedPipe)
}

// Client provides an interface for sending commands to the IPC server.
type Client struct {
	socketPath string
}

// NewClient creates a new IPC client instance.
func NewClient() *Client {
	return &Client{
		socketPath: SocketPath(),
	}
}

// SocketPath returns the path to the IPC socket.
func (c *Client) SocketPath() string {
	return c.socketPath
}

// Send transmits a command to the IPC server using the default timeout.
func (c *Client) Send(cmd Command) (Response, error) {
	return c.SendWithTimeout(cmd, DefaultTimeout)
}

// SendWithTimeout transmits a command to the IPC server with a specified timeout.
func (c *Client) SendWithTimeout(cmd Command, timeout time.Duration) (Response, error) {
	// Create a dialer with timeout
	dialer := net.Dialer{
		Timeout: ConnectionTimeout,
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	connection, connectionErr := dialEndpoint(ctx, dialer, c.socketPath)
	if connectionErr != nil {
		if derrors.Is(ctx.Err(), context.DeadlineExceeded) {
			return Response{}, derrors.New(
				derrors.CodeTimeout,
				"connection timeout: neru may be unresponsive",
			)
		}

		return Response{}, derrors.Wrap(
			connectionErr,
			derrors.CodeIPCFailed,
			connectFailureMessage(),
		)
	}

	var closeErr error

	defer func() {
		connectionCloseErr := connection.Close()
		if connectionCloseErr != nil && closeErr == nil {
			closeErr = derrors.Wrap(
				connectionCloseErr,
				derrors.CodeIPCFailed,
				"failed to close connection",
			)
		}
	}()

	connectionDeadlineErr := connection.SetDeadline(time.Now().Add(timeout))
	if connectionDeadlineErr != nil {
		return Response{}, derrors.Wrap(
			connectionDeadlineErr,
			derrors.CodeIPCFailed,
			"failed to set connection deadline",
		)
	}

	encoder := json.NewEncoder(connection)
	decoder := json.NewDecoder(connection)

	if cmd.Version == "" {
		cmd.Version = BuildVersion()
	}

	encodeErr := encoder.Encode(cmd)
	if encodeErr != nil {
		if derrors.Is(ctx.Err(), context.DeadlineExceeded) {
			return Response{}, derrors.New(
				derrors.CodeTimeout,
				"send timeout: neru may be unresponsive",
			)
		}

		var wrapped error = derrors.Wrap(encodeErr, derrors.CodeIPCFailed, "failed to send command")
		if closeErr != nil {
			wrapped = derrors.Wrapf(
				wrapped,
				derrors.CodeIPCFailed,
				"%v (close error: %s)",
				wrapped,
				closeErr.Error(),
			)
		}

		return Response{}, wrapped
	}

	var response Response

	decodeErr := decoder.Decode(&response)
	if decodeErr != nil {
		if derrors.Is(ctx.Err(), context.DeadlineExceeded) {
			return Response{}, derrors.New(
				derrors.CodeTimeout,
				"receive timeout: neru may be unresponsive",
			)
		}

		var wrapped error = derrors.Wrap(decodeErr, derrors.CodeIPCFailed, "failed to receive response")
		if closeErr != nil {
			wrapped = derrors.Wrapf(
				wrapped,
				derrors.CodeIPCFailed,
				"%v (close error: %s)",
				wrapped,
				closeErr.Error(),
			)
		}

		return Response{}, wrapped
	}

	if closeErr != nil {
		return response, closeErr
	}

	return response, nil
}

// connectFailureMessage explains a connection that never got off the ground,
// adding whatever the transport can say about where else a daemon might be.
func connectFailureMessage() string {
	message := "failed to connect to neru (is it running?)"

	hint := endpointHint()
	if hint != "" {
		message += "; " + hint
	}

	return message
}

// IsServerRunning determines if the IPC server is currently accepting connections.
// It returns true even when the daemon has a different build version — the
// version mismatch error will surface when the actual command is sent.
func IsServerRunning() bool {
	client := NewClient()

	// We get a response (possibly a version-mismatch error) if the server is up.
	// A transport-level error (connection refused, timeout) means it's not running.
	_, err := client.SendWithTimeout(Command{Action: "ping"}, PingTimeout)

	return err == nil
}
