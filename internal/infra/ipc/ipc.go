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

	"github.com/y3owk1n/neru/internal/domain/trace"
	"go.uber.org/zap"
)

const (
	// SocketName is the name of the Unix socket file.
	SocketName = "neru.sock"

	// DefaultTimeout is the default timeout for IPC operations.
	DefaultTimeout = 5 * time.Second

	// ConnectionTimeout is the timeout for establishing a connection.
	ConnectionTimeout = 2 * time.Second
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
	Action string         `json:"action"`
	Params map[string]any `json:"params,omitempty"`
	Args   []string       `json:"args,omitempty"`
}

// Response represents a response returned through the IPC interface.
type Response struct {
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

// GetSocketPath returns the filesystem path to the IPC Unix socket.
func GetSocketPath() string {
	tmpDir := os.TempDir()
	return filepath.Join(tmpDir, SocketName)
}

// NewServer initializes a new IPC server instance with the specified handler.
func NewServer(handler CommandHandler, logger *zap.Logger) (*Server, error) {
	socketPath := GetSocketPath()

	// Remove existing socket if it exists
	err := os.Remove(socketPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to remove existing socket: %w", err)
	}

	// Create a ListenConfig with context support
	listenConfig := &net.ListenConfig{}
	listener, err := listenConfig.Listen(context.Background(), "unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create socket: %w", err)
	}

	updateSocketPermissionsErr := os.Chmod(socketPath, 0o600)
	if updateSocketPermissionsErr != nil {
		_ = listener.Close()
		return nil, fmt.Errorf("failed to set socket permissions: %w", updateSocketPermissionsErr)
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
			conn, err := s.listener.Accept()
			if err != nil {
				// If listener is closed, exit gracefully
				if errors.Is(err, net.ErrClosed) {
					s.logger.Info("IPC server listener closed, stopping accept loop")
					return
				}
				s.logger.Error("Failed to accept connection", zap.Error(err))
				continue
			}

			s.wg.Add(1)
			go s.handleConnection(conn)
		}
	}()
}

// Stop terminates the IPC server and cleans up resources.
func (s *Server) Stop() error {
	if s.listener != nil {
		err := s.listener.Close()
		if err != nil {
			return fmt.Errorf("failed to close listener: %w", err)
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
		timer.Stop() // Stop timer immediately on success
		// All connections closed successfully
	case <-timer.C:
		// Timeout waiting for connections to close
		s.logger.Warn("IPC server: timeout waiting for connections to close")
	}

	// Clean up socket file
	err := os.Remove(s.socketPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove socket file: %w", err)
	}

	return nil
}

// handleConnection processes a single client connection and executes the received command.
func (s *Server) handleConnection(conn net.Conn) {
	traceID := trace.NewID()
	log := s.logger.With(zap.String("trace_id", traceID.String()))

	// Create context with trace ID
	ctx := trace.WithTraceID(context.Background(), traceID)

	defer func() {
		err := conn.Close()
		if err != nil {
			log.Error("Failed to close connection", zap.Error(err))
		}
		s.wg.Done()
	}()

	// Set read deadline to prevent hanging connections
	err := conn.SetDeadline(time.Now().Add(30 * time.Second))
	if err != nil {
		log.Error("Failed to set connection deadline", zap.Error(err))
		return
	}

	decoder := json.NewDecoder(conn)
	decoder.DisallowUnknownFields()
	encoder := json.NewEncoder(conn)

	var cmd Command
	err = decoder.Decode(&cmd)
	if err != nil {
		log.Error("Failed to decode command", zap.Error(err))
		encErr := encoder.Encode(Response{
			Success: false,
			Message: fmt.Sprintf("failed to decode command: %v", err),
			Code:    CodeInvalidInput,
		})
		if encErr != nil {
			log.Error("Failed to encode error response", zap.Error(encErr))
		}
		return
	}

	log.Info("Received command", zap.String("action", cmd.Action))

	response := s.handler(ctx, cmd)
	err = encoder.Encode(response)
	if err != nil {
		log.Error("Failed to encode response", zap.Error(err))
	}
}

// Client provides an interface for sending commands to the IPC server.
type Client struct {
	socketPath string
}

// NewClient initializes a new IPC client instance.
func NewClient() *Client {
	return &Client{
		socketPath: GetSocketPath(),
	}
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

	conn, err := dialer.DialContext(ctx, "unix", c.socketPath)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return Response{}, errors.New("connection timeout: neru may be unresponsive")
		}
		return Response{}, fmt.Errorf("failed to connect to neru (is it running?): %w", err)
	}

	var closeErr error
	defer func() {
		err := conn.Close()
		if err != nil && closeErr == nil {
			closeErr = fmt.Errorf("failed to close connection: %w", err)
		}
	}()

	// Set deadline for the entire operation
	err = conn.SetDeadline(time.Now().Add(timeout))
	if err != nil {
		return Response{}, fmt.Errorf("failed to set connection deadline: %w", err)
	}

	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	err = encoder.Encode(cmd)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return Response{}, errors.New("send timeout: neru may be unresponsive")
		}
		err = fmt.Errorf("failed to send command: %w", err)
		if closeErr != nil {
			err = fmt.Errorf("%w (close error: %s)", err, closeErr.Error())
		}
		return Response{}, err
	}

	var response Response
	err = decoder.Decode(&response)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return Response{}, errors.New("receive timeout: neru may be unresponsive")
		}
		err = fmt.Errorf("failed to receive response: %w", err)
		if closeErr != nil {
			err = fmt.Errorf("%w (close error: %s)", err, closeErr.Error())
		}
		return Response{}, err
	}

	if closeErr != nil {
		return response, closeErr
	}
	return response, nil
}

// IsServerRunning determines if the IPC server is currently accepting connections.
func IsServerRunning() bool {
	client := NewClient()
	_, err := client.SendWithTimeout(Command{Action: "ping"}, 500*time.Millisecond)
	return err == nil
}
