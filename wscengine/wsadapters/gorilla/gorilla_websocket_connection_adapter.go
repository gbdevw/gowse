// Package which contains a WebsocketConnectionAdapterInterface implementation for
// gorilla/websocket library (https://github.com/gorilla/websocket).
package wsadapternhooyr

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	wsconnadapter "github.com/gbdevw/gowsclient/wscengine/wsadapters"
	"github.com/gorilla/websocket"
)

// Adapter for gorilla/websocket library
type GorillaWebsocketConnectionAdapter struct {
	// Undelrying websocket connection
	conn *websocket.Conn
	// Dial options to use when opening a connection
	dialer *websocket.Dialer
	// Headers to use when opening a connection
	requestHeader http.Header
	// Internal mutex
	mu sync.Mutex
	// Internal channel of channels used to manage ping/pong
	//
	// The channel that is sent is used to wait for pong.
	pingRequests chan chan any
}

// # Description
//
// Factory which creates a new GorillaWebsocketConnectionAdapter.
//
// # Inputs
//
//   - dialer: Optional dialer to use when using Dial method. If nil, the default dialer
//     defined by gorilla library will be used.
//
//   - requestHeader: Headers which will be used during Dial to specify the origin (Origin),
//     subprotocols (Sec-WebSocket-Protocol) and cookies (Cookie)
//
// # Returns
//
// New GorillaWebsocketConnectionAdapter
func NewGorillaWebsocketConnectionAdapter(dialer *websocket.Dialer, requestHeader http.Header) *GorillaWebsocketConnectionAdapter {
	if dialer == nil {
		// Use default dialer if nil
		dialer = websocket.DefaultDialer
	}
	// Build and return adapter
	return &GorillaWebsocketConnectionAdapter{
		conn:          nil,
		dialer:        dialer,
		requestHeader: requestHeader,
		mu:            sync.Mutex{},
		// Use a chan with capacity so ping requests can be recorded before sending ping message.
		pingRequests: make(chan chan any, 10),
	}
}

// # Description
//
// Dial opens a connection to the websocket server and performs a WebSocket handshake.
//
// # Expected behaviour
//
//   - Dial MUST block until websocket handshake is complete. Websocket handshake and TLS must
//     be handled seamlessly either by the adapter implementation or by the underlying websocket
//     library.
//
//   - Dial MUST NOT return the underlying websocket connection. The undelrying websocket
//     connection must be kept internally by the adapter implementation in order to be used
//     later by other adapter methods.
//
//   - Dial MUST return an error in case a connection has already been established and Close
//     method has not been called yet.
//
// # Inputs
//
//   - ctx: Context used for tracing/timeout purpose
//   - target: Target server URL
//
// # Returns
//
// The server response to websocket handshake or an error if any.
func (adapter *GorillaWebsocketConnectionAdapter) Dial(ctx context.Context, target url.URL) (*http.Response, error) {
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
		conn, res, err := adapter.dialer.DialContext(ctx, target.String(), adapter.requestHeader)
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
// Send a close message with the provided status code and an optional close reason and close
// the websocket connection.
//
// # Expected behaviour
//
//   - Close MUST be blocking until close message has been sent to the server.
//   - Close MUST drop pending write/read messages.
//   - Close MUST return a (wrapped) net.ErrClosed error in case connection is already closed.
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
func (adapter *GorillaWebsocketConnectionAdapter) Close(ctx context.Context, code wsconnadapter.StatusCode, reason string) error {
	// Lock internal mutex before accessing internal state
	adapter.mu.Lock()
	defer adapter.mu.Unlock()
	// Check whether there is already a connection set
	if adapter.conn == nil {
		return fmt.Errorf("close failed because no connection is already up")
	}
	// Close connection
	err := adapter.conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(int(code), reason), time.Now().Add(60*time.Second))
	// Void connection in any case
	adapter.conn = nil
	// Return result
	return err
}

// # Description
//
// Send a Ping message to the websocket server and blocks until a Pong response is received.
//
// # Expected behaviour
//
//   - Ping MUST be blocking either until an error or a context timeout or cancellation occurs
//     or until Ping message is sent and a Pong response is somehow detected either by the
//     adapter implementation or by the underlying websocket connection library.
//
//   - It CANNOT be assumed that there will be at least one concurrent goroutine which continuously
//     call Read method. In case the underlying websocket library requires to have a concurrent
//     goroutine continuously reading in order for Ping to complete, it is up to either the
//     adapter or to the final user to ensure there is a concurrent goroutine reading.
//
//   - Ping MUST return an error if connection is closed, if server is unreachable or if context
//     has expired (timeout or cancel). In this later case, Ping MUST return the context error.
//
// # Inputs
//
//   - ctx: context used for tracing/timeout purpose.
//
// # Returns
//
// - nil in case of success: if a Ping message is sent to the server and if a Pong is received.
// - error: connection is closed, context timeout/cancellation, ...
func (adapter *GorillaWebsocketConnectionAdapter) Ping(ctx context.Context) error {
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
		// Create channel to receive pong and send it on pingRequest channel
		// It is OK because pingRequest is a channel with capacity
		// pong channel must be a blocking channel
		pong := make(chan any)
		adapter.pingRequests <- pong
		// Send Ping
		err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(60*time.Second))
		if err != nil {
			return fmt.Errorf("ping failed: %w", err)
		}
		// Wait for a pong or for ctx cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-pong:
			return nil
		}
	}
}

// # Description
//
// Read a single message from the websocket server. Read blocks until a message is received
// from the server, until connection closes or until a timeout or a cancel occurs.
//
// # Expected behaviour
//
//   - Read MUST handle seamlessly message defragmentation, decompression and TLS decryption.
//     It is up to the adapter implementation or to the underlying websocket library to handle
//     these features.
//
//   - Read MUST NOT return close, ping, pong and continuation frames as control frames MUST be
//     handled seamlessly either by the adapter implementation or by the underlying websocket
//     connection library.
//
//   - Read MUST return a WebsocketCloseError either if a close message is read or if connection
//     is closed without a close message. In the later case, the 1006 status code MUST be used.
//
//   - Read MUST block until a message is read from the server or until an error occurs.
//
// # Inputs
//
//   - ctx: Context used for tracing/timeout purpose
//
// # Returns
//
//   - MessageType: received message type (Binary | Text)
//   - []bytes: Message content
//   - error: in case of connection closure, context timeout/cancellation or failure.
func (adapter *GorillaWebsocketConnectionAdapter) Read(ctx context.Context) (wsconnadapter.MessageType, []byte, error) {
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
		// Call Read until a message is read - handle control frames
		for {
			// Read message
			msgType, msg, err := conn.ReadMessage()
			if err != nil {
				// Check if close error
				if ce, ok := err.(*websocket.CloseError); ok {
					// Connection is closed
					return -1, nil, wsconnadapter.WebsocketCloseError{
						Code:   wsconnadapter.StatusCode(ce.Code),
						Reason: err.Error(),
						Err:    err,
					}
				}
				if errors.Is(err, io.EOF) ||
					strings.Contains(strings.ToLower(err.Error()), "use of closed network connection") {
					// Connection is closed
					return -1, nil, wsconnadapter.WebsocketCloseError{
						Code:   wsconnadapter.AbnormalClosure,
						Reason: err.Error(),
						Err:    err,
					}
				}
				// Other errors
				return -1, nil, err
			}
			// Check message type
			switch msgType {
			case websocket.CloseMessage:
				// Close message from server - Force close connection
				adapter.conn.Close()
				// Return close error with received data
				return -1, nil, wsconnadapter.WebsocketCloseError{
					Code:   wsconnadapter.StatusCode(binary.BigEndian.Uint16(msg[0:2])),
					Reason: string(msg[2:]),
					Err:    err,
				}
			case websocket.PongMessage:
				// Pong - browse ping requests until a channel with an active listener
				// is found to send the pong signal. Discard if no active pong listener.
				for {
					select {
					// Extract a pending ping request if any
					case pong := <-adapter.pingRequests:
						select {
						// Try to write pong signal on blocking channel
						case pong <- nil:
							// Ok - break
						default:
							// No active listener on this ping request
							// Proceed to next pending ping request
							continue
						}
					default:
						// No pending ping request - break
					}
					// Break loop
					break
				}
			case websocket.PingMessage:
				// Discard
				continue
			default:
				// Return message
				return wsconnadapter.MessageType(msgType), msg, nil
			}
		}
	}
}

// # Description
//
// Write a single message to the websocket server. Write blocks until message is sent to the
// server or until an error occurs: context timeout, cancellation, connection closed, ....
//
// # Expected behaviour
//
//   - Write MUST handle seamlessly message fragmentation, compression and TLS encryption. It is
//     up to the adapter implementation or to the underlying websocket library to handle these.
//
//   - Write MUST NOT handle sending control frames like Close, Ping, etc...
//
//   - Write MUST be blocking until a message is sent to the server or until an error occurs.
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
func (adapter *GorillaWebsocketConnectionAdapter) Write(ctx context.Context, msgType wsconnadapter.MessageType, msg []byte) error {
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
		return conn.WriteMessage(int(msgType), msg)
	}
}

// # Description
//
// Return the underlying websocket connection if any. Returned value has to be type asserted.
//
// # Returns
//
// The underlying websocket connection if any. Returned value has to be type asserted.
func (adapter *GorillaWebsocketConnectionAdapter) GetUnderlyingWebsocketConnection() any {
	// Lock internal mutex before accessing internal state
	adapter.mu.Lock()
	defer adapter.mu.Unlock()
	// Return underlying connection
	return adapter.conn
}
