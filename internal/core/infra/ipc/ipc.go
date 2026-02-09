package ipc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"github.com/y3owk1n/neru/internal/core/infra/trace"
	"go.uber.org/zap"
)

const (
	// SocketName is the name of the Unix socket file.
	SocketName = "neru.sock"

	// DefaultTimeout is the default timeout for IPC operations.
	DefaultTimeout = 5 * time.Second

	// ConnectionTimeout is the timeout for establishing a connection.
	ConnectionTimeout = 2 * time.Second

	// ProtocolVersion is the current IPC protocol version.
	// Increment this when making breaking changes to the IPC protocol.
	ProtocolVersion = "1.0.0"

	// ConnectionReadTimeout is the timeout for reading from a connection.
	ConnectionReadTimeout = 30 * time.Second

	// PingTimeout is the timeout for ping operations.
	PingTimeout = 500 * time.Millisecond

	// DefaultSocketPerms is the default socket permissions.
	DefaultSocketPerms = 0o600
)

// Standard response codes used to indicate the result of IPC operations.
const (
	CodeOK             = "OK"
	CodeUnknownCommand = "ERR_UNKNOWN_COMMAND"
	CodeNotRunning     = "ERR_NOT_RUNNING"
	CodeAlreadyRunning = "ERR_ALREADY_RUNNING"
	CodeModeDisabled   = "ERR_MODE_DISABLED"
	CodeInvalidInput   = "ERR_INVALID_INPUT"
	CodeActionFailed   = "ERR_ACTION_FAILED"
)

// Command represents a command sent through the IPC interface.
type Command struct {
	Version string         `json:"version,omitempty"`
	Action  string         `json:"action"`
	Params  map[string]any `json:"params,omitempty"`
	Args    []string       `json:"args,omitempty"`
}

// Response represents a response returned through the IPC interface.
type Response struct {
	Version string `json:"version,omitempty"`
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Code    string `json:"code,omitempty"`
	Data    any    `json:"data,omitempty"`
}

// StatusData represents the payload structure for status query responses.
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

// CommandHandler defines the interface for processing IPC commands.
type CommandHandler func(ctx context.Context, cmd Command) Response

// SocketPath returns the filesystem path to the IPC Unix socket.
func SocketPath() string {
	tmpDir := os.TempDir()

	return filepath.Join(tmpDir, SocketName)
}

// NewServer initializes a new IPC server instance with the specified handler.
func NewServer(handler CommandHandler, logger *zap.Logger) (*Server, error) {
	socketPath := SocketPath()

	// Remove existing socket if it exists
	removeSocketErr := os.Remove(socketPath)
	if removeSocketErr != nil && !os.IsNotExist(removeSocketErr) {
		return nil, derrors.Wrap(
			removeSocketErr,
			derrors.CodeIPCFailed,
			"failed to remove existing socket",
		)
	}

	// Create a ListenConfig with context support
	listenConfig := &net.ListenConfig{}

	listener, listenerErr := listenConfig.Listen(context.Background(), "unix", socketPath)
	if listenerErr != nil {
		return nil, derrors.Wrap(listenerErr, derrors.CodeIPCFailed, "failed to create socket")
	}

	updateSocketPermissionsErr := os.Chmod(socketPath, DefaultSocketPerms)
	if updateSocketPermissionsErr != nil {
		_ = listener.Close()

		return nil, derrors.Wrap(
			updateSocketPermissionsErr,
			derrors.CodeIPCFailed,
			"failed to set socket permissions",
		)
	}

	logger.Info("IPC server created", zap.String("socket", socketPath))

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
					s.logger.Info("IPC server listener closed, stopping accept loop")

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
	if s.listener != nil {
		closeListenerErr := s.listener.Close()
		if closeListenerErr != nil {
			return derrors.Wrap(closeListenerErr, derrors.CodeIPCFailed, "failed to close listener")
		}
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
		// Timeout waiting for connections to close
		s.logger.Warn("IPC server: timeout waiting for connections to close")
	}

	// Clean up socket file
	removeSocketFileErr := os.Remove(s.socketPath)
	if removeSocketFileErr != nil && !os.IsNotExist(removeSocketFileErr) {
		return derrors.Wrap(
			removeSocketFileErr,
			derrors.CodeIPCFailed,
			"failed to remove socket file",
		)
	}

	return nil
}

// handleConnection processes a single client connection and executes the received command.
func (s *Server) handleConnection(connection net.Conn) {
	traceID := trace.NewID()
	logger := s.logger.With(zap.String("trace_id", traceID.String()))

	// Create context with trace ID
	ctx := trace.WithTraceID(context.Background(), traceID)

	defer func() {
		connectionCloseErr := connection.Close()
		if connectionCloseErr != nil {
			logger.Error("Failed to close connection", zap.Error(connectionCloseErr))
		}

		s.wg.Done()
	}()

	connectionDeadline := connection.SetDeadline(time.Now().Add(ConnectionReadTimeout))
	if connectionDeadline != nil {
		logger.Error("Failed to set connection deadline", zap.Error(connectionDeadline))

		return
	}

	decoder := json.NewDecoder(connection)
	decoder.DisallowUnknownFields()

	encoder := json.NewEncoder(connection)

	var cmd Command

	decodeCommandErr := decoder.Decode(&cmd)
	if decodeCommandErr != nil {
		logger.Error("Failed to decode command", zap.Error(decodeCommandErr))

		encodeErr := encoder.Encode(Response{
			Success: false,
			Message: fmt.Sprintf("failed to decode command: %v", decodeCommandErr),
			Code:    CodeInvalidInput,
		})
		if encodeErr != nil {
			logger.Error("Failed to encode error response", zap.Error(encodeErr))
		}

		return
	}

	logger.Info(
		"Received command",
		zap.String("action", cmd.Action),
		zap.String("version", cmd.Version),
	)

	// Validate protocol version if provided
	if cmd.Version != "" && cmd.Version != ProtocolVersion {
		logger.Warn("Protocol version mismatch",
			zap.String("client_version", cmd.Version),
			zap.String("server_version", ProtocolVersion))

		encodeErr := encoder.Encode(Response{
			Version: ProtocolVersion,
			Success: false,
			Message: fmt.Sprintf(
				"protocol version mismatch: client=%s, server=%s",
				cmd.Version,
				ProtocolVersion,
			),
			Code: "ERR_VERSION_MISMATCH",
		})
		if encodeErr != nil {
			logger.Error("Failed to encode version mismatch response", zap.Error(encodeErr))
		}

		return
	}

	response := s.handler(ctx, cmd)
	// Always include server version in response
	response.Version = ProtocolVersion

	connectionDeadline = encoder.Encode(response)
	if connectionDeadline != nil {
		logger.Error("Failed to encode response", zap.Error(connectionDeadline))
	}
}

// Client provides an interface for sending commands to the IPC server.
type Client struct {
	socketPath string
}

// NewClient initializes a new IPC client instance.
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

	connection, connectionErr := dialer.DialContext(ctx, "unix", c.socketPath)
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
			"failed to connect to neru (is it running?)",
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
		cmd.Version = ProtocolVersion
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

// IsServerRunning determines if the IPC server is currently accepting connections.
func IsServerRunning() bool {
	client := NewClient()
	_, err := client.SendWithTimeout(Command{Action: "ping"}, PingTimeout)

	return err == nil
}
