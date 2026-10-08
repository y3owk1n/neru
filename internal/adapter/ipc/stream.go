package ipc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/derrors"
)

// maxStreams caps how many streams the daemon keeps open at once. A stream
// holds a connection and a goroutine for as long as its client stays, so the
// cap keeps a client that opens them in a loop from holding the daemon's.
const maxStreams = 16

// StreamHandler serves an action whose reply is a stream. It writes each value
// through emit, and returns when ctx ends, which it does once the client goes
// away or the server stops, or when emit fails.
//
// emit is called from the handler's own goroutine. A handler that returns an
// error before its first emit refuses the stream, and the client gets that
// error as its reply. After the first emit, an error only ends the stream.
type StreamHandler func(ctx context.Context, cmd Command, emit func(value any) error) error

// HandleStream registers handler for action. Call it before Start.
func (s *Server) HandleStream(action string, handler StreamHandler) {
	if s.streamHandlers == nil {
		s.streamHandlers = make(map[string]StreamHandler)
	}

	s.streamHandlers[action] = handler
}

// serveStream answers a stream action on connection. It replies OK just
// before the first value, so a stream that cannot start answers with its error
// instead, and it returns once the stream ends.
func (s *Server) serveStream(
	ctx context.Context,
	connection net.Conn,
	encoder *json.Encoder,
	cmd Command,
	handler StreamHandler,
	reply func(Response),
	logger *zap.Logger,
) {
	if refusal := s.addStream(connection); refusal != nil {
		reply(*refusal)

		return
	}
	defer s.removeStream(connection)

	// A stream outlives the read deadline that guards a client that never
	// sends its command.
	deadlineErr := connection.SetReadDeadline(time.Time{})
	if deadlineErr != nil {
		logger.Debug("Failed to clear the stream's read deadline", zap.Error(deadlineErr))

		return
	}

	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// The client sends nothing after its command, so a read returns only when
	// the client goes away or Stop closes the connection. Either ends the
	// stream, which makes it the one way a stream ends.
	go func() {
		_, _ = io.Copy(io.Discard, connection)

		cancel()
	}()

	started := false
	emit := func(value any) error {
		if !started {
			reply(
				Response{
					Version: BuildVersion(),
					Success: true,
					Message: "streaming",
					Code:    CodeOK,
				},
			)

			started = true
		}

		writeErr := connection.SetWriteDeadline(time.Now().Add(ConnectionWriteTimeout))
		if writeErr != nil {
			return writeErr
		}

		return encoder.Encode(value)
	}

	handlerErr := handler(streamCtx, cmd, emit)

	switch {
	case handlerErr == nil, isPeerGoneErr(handlerErr), streamCtx.Err() != nil:
		logger.Debug("Stream ended", zap.String("action", cmd.Action))
	case !started:
		reply(Response{
			Version: BuildVersion(),
			Success: false,
			Message: handlerErr.Error(),
			Code:    responseCode(handlerErr),
		})
	default:
		logger.Debug("Stream ended on a failed write",
			zap.String("action", cmd.Action), zap.Error(handlerErr))
	}
}

// responseCode names a refused stream's error on the wire.
func responseCode(err error) string {
	if derrors.IsNotSupported(err) {
		return CodeNotSupported
	}

	return CodeActionFailed
}

// addStream records connection as an open stream, or returns the reply that
// refuses it: when maxStreams are already open, or when Stop has already
// closed the open ones, which a stream added after would outlive.
func (s *Server) addStream(connection net.Conn) *Response {
	s.streamsMu.Lock()
	defer s.streamsMu.Unlock()

	if s.streamsClosed {
		return &Response{
			Version: BuildVersion(),
			Success: false,
			Message: "the daemon is stopping",
			Code:    CodeActionFailed,
		}
	}

	if len(s.streams) >= maxStreams {
		return &Response{
			Version: BuildVersion(),
			Success: false,
			Message: fmt.Sprintf("at most %d streams can be open at once", maxStreams),
			Code:    CodeBusy,
		}
	}

	if s.streams == nil {
		s.streams = make(map[net.Conn]struct{})
	}

	s.streams[connection] = struct{}{}

	return nil
}

// removeStream forgets a stream that has ended.
func (s *Server) removeStream(connection net.Conn) {
	s.streamsMu.Lock()
	defer s.streamsMu.Unlock()

	delete(s.streams, connection)
}

// closeStreams ends every open stream by closing its connection, which ends
// its reader and so its handler, and refuses any stream asked for after.
// streamsMu is a leaf, held only across the closes.
func (s *Server) closeStreams() {
	s.streamsMu.Lock()
	defer s.streamsMu.Unlock()

	s.streamsClosed = true

	for connection := range s.streams {
		_ = connection.Close()
	}
}

// Stream sends cmd and, once the daemon accepts it, hands each line after the
// reply to each, until the daemon closes the connection or each fails. timeout
// bounds the connect and the reply only, never the stream.
//
// A refused stream returns its reply and no error. A stream the daemon ends,
// as it does when it exits, returns the accepting reply and no error.
func (c *Client) Stream(
	cmd Command,
	timeout time.Duration,
	each func(line json.RawMessage) error,
) (Response, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	connection, dialErr := dialEndpoint(ctx, net.Dialer{Timeout: ConnectionTimeout}, c.socketPath)
	if dialErr != nil {
		return Response{}, derrors.Wrap(dialErr, derrors.CodeIPCFailed, connectFailureMessage())
	}
	defer func() { _ = connection.Close() }()

	deadlineErr := connection.SetDeadline(time.Now().Add(timeout))
	if deadlineErr != nil {
		return Response{}, derrors.Wrap(
			deadlineErr,
			derrors.CodeIPCFailed,
			"failed to set connection deadline",
		)
	}

	if cmd.Version == "" {
		cmd.Version = BuildVersion()
	}

	encodeErr := json.NewEncoder(connection).Encode(cmd)
	if encodeErr != nil {
		return Response{}, derrors.Wrap(encodeErr, derrors.CodeIPCFailed, "failed to send command")
	}

	decoder := json.NewDecoder(connection)

	var response Response

	decodeErr := decoder.Decode(&response)
	if decodeErr != nil {
		return Response{}, derrors.Wrap(
			decodeErr,
			derrors.CodeIPCFailed,
			"failed to receive response",
		)
	}

	if !response.Success {
		return response, nil
	}

	clearErr := connection.SetDeadline(time.Time{})
	if clearErr != nil {
		return response, derrors.Wrap(
			clearErr,
			derrors.CodeIPCFailed,
			"failed to clear connection deadline",
		)
	}

	for {
		var line json.RawMessage

		lineErr := decoder.Decode(&line)
		if errors.Is(lineErr, io.EOF) || isPeerGoneErr(lineErr) {
			return response, nil
		}

		if lineErr != nil {
			return response, derrors.Wrap(
				lineErr,
				derrors.CodeIPCFailed,
				"failed to read the stream",
			)
		}

		eachErr := each(line)
		if eachErr != nil {
			return response, eachErr
		}
	}
}
