// Package which contains a WebsocketConnectionAdapterInterface implementation for
// nhooyr/websocket library (https://github.com/nhooyr/websocket).
package wsadapternhooyr

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"

	wsconnadapter "github.com/gbdevw/gowsclient/wscengine/wsadapters"
	"nhooyr.io/websocket"
)

// Adapter for nhooyr/websocket library
type NhooyrWebsocketConnectionAdapter struct {
	// Undelrying websocket connection
	conn *websocket.Conn
	// Dial options to use when opening a connection
	opts *websocket.DialOptions
	// Internal mutex
	mu sync.Mutex
}

// # Description
//
// Factory which creates a new NhooyrWebsocketConnectionAdapter.
//
// # Inputs
//
//   - opts: Optional dial options to use when calling Dial method. Can be nil.
//
// # Returns
//
// New NhooyrWebsocketConnectionAdapter
func NewNhooyrWebsocketConnectionAdapter(opts *websocket.DialOptions) *NhooyrWebsocketConnectionAdapter {
	return &NhooyrWebsocketConnectionAdapter{
		conn: nil,
		opts: opts,
		mu:   sync.Mutex{},
	}
}

// # Description
//
// Dial opens a connection to the websocket server and performs a WebSocket handshake.
//
// # Inputs
//
//   - ctx: Context used for tracing/timeout purpose
//   - target: Target server URL
//
// # Returns
//
// The server response to websocket handshake or an error if any.
func (adapter *NhooyrWebsocketConnectionAdapter) Dial(ctx context.Context, target url.URL) (*http.Response, error) {
	select {
	case <-ctx.Done():
		// Shortcut if context is done (timeout/cancel)
		return nil, ctx.Err()
	default:
		// Lock internal mutex before accessing internal state
		adapter.mu.Lock()
		defer adapter.mu.Unlock()
		// Check whether there is already a connection set
		if adapter.conn != nil {
			// Return error in case a connection has already been set
			return nil, fmt.Errorf("a connection has already been established")
		}
		// Open websocket connection
		conn, res, err := websocket.Dial(ctx, target.String(), adapter.opts)
		if err != nil {
			// Return response and error
			return res, err
		}
		// Persist connection internally and return
		adapter.conn = conn
		return res, nil
	}
}

// # Description
//
// Send a close message with the provided status code and an optional close reason and drop
// the websocket connection.
//
// # Inputs
//
//   - ctx: Context used for tracing purpose
//   - code: Status code to use in close message
//   - reason: Optional reason joined in clsoe message. Can be empty.
//
// # Returns
//
//   - nil in case of success
//   - error: server unreachable, connection already closed, ...
func (adapter *NhooyrWebsocketConnectionAdapter) Close(ctx context.Context, code wsconnadapter.StatusCode, reason string) error {
	// Lock internal mutex before accessing internal state
	adapter.mu.Lock()
	defer adapter.mu.Unlock()
	// Check whether there is already a connection set
	if adapter.conn == nil {
		return fmt.Errorf("close failed because no connection is already up")
	}
	// Close connection
	err := adapter.conn.Close(convertToNhooyrStatusCodes(code), reason)
	// Void connection in any case
	adapter.conn = nil
	// Check error
	if err != nil {
		if err.Error() == "failed to close WebSocket: already wrote close" {
			// Change to an error which wraps a net.ErrClosed
			err = fmt.Errorf("failed to close WebSocket: %w", net.ErrClosed)
		}
	}
	// Return result
	return err
}

// # Description
//
// Send a Ping message to the websocket server and blocks until a Pong response is received, a
// timeout occurs or until connection is closed.
//
// A concrrent gorotine must call Read method so that contorl frames, pong inclded, are processed
// and ping does not hang.
//
// # Inputs
//
//   - ctx: context used for tracing/timeout purpose.
//
// # Returns
//
// - nil in case of success: if a Ping message is sent to the server and if a Pong is received.
// - error: connection is closed, context timeout/cancellation, ...
func (adapter *NhooyrWebsocketConnectionAdapter) Ping(ctx context.Context) error {
	select {
	case <-ctx.Done():
		// Shortcut if context is done (timeout/cancel)
		return ctx.Err()
	default:
		// Lock internal mutex before and store current conn reference in local variable to allow
		// other routines to perform other operations on the connection.
		adapter.mu.Lock()
		conn := adapter.conn
		adapter.mu.Unlock()
		// Check whether there is already a connection set
		if conn == nil {
			return fmt.Errorf("ping failed because no connection is already up")
		}
		// Call Ping and return results
		return conn.Ping(ctx)
	}
}

// # Description
//
// Read a single message from the websocket server. Read blocks until a message is received
// from the server or until connection closes
//
// # Inputs
//
//   - ctx: Context used for tracing purpose
//
// # Returns
//
//   - MessageType: received message type (Binary | Text)
//   - []bytes: Message content
//   - error: in case of connection closure, context timeout/cancellation or failure.
func (adapter *NhooyrWebsocketConnectionAdapter) Read(ctx context.Context) (wsconnadapter.MessageType, []byte, error) {
	select {
	case <-ctx.Done():
		// Shortcut if context is done (timeout/cancel)
		return -1, nil, ctx.Err()
	default:
		// Lock internal mutex before and store current conn reference in local variable to allow
		// other routines to perform other operations on the connection.
		adapter.mu.Lock()
		conn := adapter.conn
		adapter.mu.Unlock()
		// Check whether there is already a connection set
		if conn == nil {
			return -1, nil, fmt.Errorf("read failed because no connection is already up")
		}
		// Call Read
		nhooyrMsgType, msg, err := conn.Read(ctx)
		if err != nil {
			// Check if error is due to connection being closed
			if websocket.CloseStatus(err) != -1 || errors.Is(err, io.EOF) {
				// Error is because connection has been closed
				if websocket.CloseStatus(err) != -1 {
					// We have a close status code - return typed error
					return -1, nil, wsconnadapter.WebsocketCloseError{
						Code:   convertFromNhooyrStatusCodes(websocket.CloseStatus(err)),
						Reason: err.Error(),
						Err:    err,
					}
				} else {
					// We do not have close status -> use default 1006 for typed error
					return -1, nil, wsconnadapter.WebsocketCloseError{
						Code:   wsconnadapter.AbnormalClosure,
						Reason: "websocket connection abnormal closure",
						Err:    err,
					}
				}
			} else {
				// Error is not because connection was closed
				return -1, nil, err
			}
		}
		// Return result with converted msgtype
		return convertFromNhooyrMsgTypes(nhooyrMsgType), msg, nil
	}
}

// # Description
//
// Write a single message to the websocket server. Write blocks until message is sent to the
// server or until an error occurs: context timeout, cancellation, connection closed, ....
//
// # Inputs
//
//   - ctx: Context used for tracing/timeout purpose
//   - MessageType: received message type (Binary | Text)
//   - []bytes: Message content
//
// # Returns
//
//   - error: in case of connection closure, context timeout/cancellation or failure.
func (adapter *NhooyrWebsocketConnectionAdapter) Write(ctx context.Context, msgType wsconnadapter.MessageType, msg []byte) error {
	select {
	case <-ctx.Done():
		// Shortcut if context is done (timeout/cancel)
		return ctx.Err()
	default:
		// Lock internal mutex before and store current conn reference in local variable to allow
		// other routines to perform other operations on the connection.
		adapter.mu.Lock()
		conn := adapter.conn
		adapter.mu.Unlock()
		// Check whether there is already a connection set
		if conn == nil {
			return fmt.Errorf("write failed because no connection is already up")
		}
		// Call Write and retuurn results
		return conn.Write(ctx, convertToNhooyrMsgTypes(msgType), msg)
	}
}

// # Description
//
// Return the underlying websocket connection if any. Returned value has to be type asserted.
//
// # Returns
//
// The underlying websocket connection if any. Returned value has to be type asserted.
func (adapter *NhooyrWebsocketConnectionAdapter) GetUnderlyingWebsocketConnection() any {
	// Lock internal mutex before accessing internal state
	adapter.mu.Lock()
	defer adapter.mu.Unlock()
	// Return underlying connection
	return adapter.conn
}

/*************************************************************************************************/
/* UTILS                                                                                         */
/*************************************************************************************************/

// # Description
//
// Convert a status code to nhooyr enum.
//
// # input
//
//   - code: Status code to convert
//
// # Returns
//
// Converted code or websocket.StatusAbnormalClosure if none is corresponding.
func convertToNhooyrStatusCodes(code wsconnadapter.StatusCode) websocket.StatusCode {
	if code == wsconnadapter.NormalClosure {
		return websocket.StatusNormalClosure
	}
	if code == wsconnadapter.GoingAway {
		return websocket.StatusGoingAway
	}
	if code == wsconnadapter.ProtocolError {
		return websocket.StatusProtocolError
	}
	if code == wsconnadapter.UnsupportedData {
		return websocket.StatusUnsupportedData
	}
	if code == wsconnadapter.NoStatusReceived {
		return websocket.StatusNoStatusRcvd
	}
	if code == wsconnadapter.InvalidFramePayloadData {
		return websocket.StatusInvalidFramePayloadData
	}
	if code == wsconnadapter.PolicyViolation {
		return websocket.StatusPolicyViolation
	}
	if code == wsconnadapter.MessageTooBig {
		return websocket.StatusMessageTooBig

	}
	if code == wsconnadapter.MandatoryExtension {
		return websocket.StatusMandatoryExtension
	}
	if code == wsconnadapter.InternalError {
		return websocket.StatusInternalError
	}
	if code == wsconnadapter.TLSHandshake {
		return websocket.StatusTLSHandshake
	}
	return websocket.StatusAbnormalClosure
}

// # Description
//
// Convert a status code from nhooyr enum.
//
// # input
//
//   - code: Status code to convert
//
// # Returns
//
// Converted code or wsconnadapter.AbnormalClosure if none is corresponding.
func convertFromNhooyrStatusCodes(code websocket.StatusCode) wsconnadapter.StatusCode {
	if code == websocket.StatusNormalClosure {
		return wsconnadapter.NormalClosure
	}
	if code == websocket.StatusGoingAway {
		return wsconnadapter.GoingAway
	}
	if code == websocket.StatusProtocolError {
		return wsconnadapter.ProtocolError
	}
	if code == websocket.StatusUnsupportedData {
		return wsconnadapter.UnsupportedData
	}
	if code == websocket.StatusNoStatusRcvd {
		return wsconnadapter.NoStatusReceived
	}
	if code == websocket.StatusInvalidFramePayloadData {
		return wsconnadapter.InvalidFramePayloadData
	}
	if code == websocket.StatusPolicyViolation {
		return wsconnadapter.PolicyViolation
	}
	if code == websocket.StatusMessageTooBig {
		return wsconnadapter.MessageTooBig

	}
	if code == websocket.StatusMandatoryExtension {
		return wsconnadapter.MandatoryExtension
	}
	if code == websocket.StatusInternalError {
		return wsconnadapter.InternalError
	}
	if code == websocket.StatusTLSHandshake {
		return wsconnadapter.TLSHandshake
	}
	return wsconnadapter.AbnormalClosure
}

// # Description
//
// Convert messages types from MessageType to Nhooyr specific types.
//
// # Inputs
//
//   - msgType: code to convert
//
// # Returns
//
// Converted code. Default to binary message type if no match.
func convertToNhooyrMsgTypes(msgType wsconnadapter.MessageType) websocket.MessageType {
	if msgType == wsconnadapter.Text {
		return websocket.MessageText
	}
	return websocket.MessageBinary
}

// # Description
//
// Convert messages types from Nhooyr specific types to MessageType.
//
// # Inputs
//
//   - msgType: code to convert
//
// # Returns
//
// Converted code. Default to binary message type if no match.
func convertFromNhooyrMsgTypes(msgType websocket.MessageType) wsconnadapter.MessageType {
	if msgType == websocket.MessageText {
		return wsconnadapter.Text
	}
	return wsconnadapter.Binary
}
