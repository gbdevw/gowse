// The package defines an interface to adapt 3rd parties websocket libraries to websocket engine.
package wsadapters

import (
	"context"
	"net/http"
	"net/url"
)

// Interface which describes the adapter methods and behaviour that the websocket engine expects
// from the underlying websocket connection library.
//
// Adapters are assumed to be thread-safe. Thread safety must be ensured either by the adapter
// implementation or by the underlying websocket connection library.
type WebsocketConnectionAdapterInterface interface {
	// # Description
	//
	// Dial opens a connection to the websocket server and performs a WebSocket handshake.
	//
	// # Expected behaviour
	//
	//	- Dial MUST block until websocket handshake is complete. Websocket handshake and TLS must
	//	  be handled seamlessly either by the adapter implementation or by the underlying websocket
	//    library.
	//
	//	- Dial MUST NOT return the underlying websocket connection. The undelrying websocket
	//    connection must be kept internally by the adapter implementation in order to be used
	//    later by other adapter methods.
	//
	//	- Dial MUST return an error in case a connection has already been established and Close
	//	  method has not been called yet.
	//
	// # Inputs
	//
	//	- ctx: Context used for tracing/timeout purpose
	//	- target: Target server URL
	//
	// # Returns
	//
	// The server response to websocket handshake or an error if any.
	Dial(ctx context.Context, target url.URL) (*http.Response, error)
	// # Description
	//
	// Send a close message with the provided status code and an optional close reason and drop
	// the websocket connection.
	//
	// # Expected behaviour
	//
	//	- Close MUST be blocking until close message has been sent to the server.
	//	- Close MUST drop pending write/read messages.
	//	- Close MUST return a (wrapped) net.ErrClosed error in case connection is already closed.
	//
	// # Inputs
	//
	//	- ctx: Context used for tracing purpose
	//	- code: Status code to use in close message
	//	- reason: Optional reason joined in clsoe message. Can be empty.
	//
	// # Returns
	//
	//	- nil in case of success
	//	- error: server unreachable, connection already closed, ...
	Close(ctx context.Context, code StatusCode, reason string) error
	// # Description
	//
	// Send a Ping message to the websocket server and blocks until a Pong response is received, a
	// timmeout occurs, or connection is closed.
	//
	// # Expected behaviour
	//
	//	- Ping MUST be blocking either until an error or a context timeout or cancellation occurs
	//    or until Ping message is sent and a Pong response is somehow detected either by the
	//    adapter implementation or by the underlying websocket connection library.
	//
	//	- It CANNOT be assumed that there will be at least one concurrent goroutine which continuously
	//    call Read method. In case the underlying websocket library requires to have a concurrent
	//    goroutine continuously reading in order for Ping to complete, it is up to either the
	//    adapter or to the final user to ensure there is a concurrent goroutine reading.
	//
	//	- Ping MUST return an error if connection is closed, if server is unreachable or if context
	//    has expired (timeout or cancel). In this later case, Ping MUST return the context error.
	//
	// # Inputs
	//
	//	- ctx: context used for tracing/timeout purpose.
	//
	// # Returns
	//
	// - nil in case of success: if a Ping message is sent to the server and if a Pong is received.
	// - error: connection is closed, context timeout/cancellation, ...
	Ping(ctx context.Context) error
	// # Description
	//
	// Read a single message from the websocket server. Read blocks until a message is received
	// from the server or until connection closes.
	//
	// # Expected behaviour
	//
	//	- Read MUST handle seamlessly message defragmentation, decompression and TLS decryption.
	//    It is up to the adapter implementation or to the underlying websocket library to handle
	//    these features.
	//
	//	- Read MUST NOT return close, ping, pong and continuation frames as control frames MUST be
	//    handled seamlessly either by the adapter implementation or by the underlying websocket
	//    connection library.
	//
	//	- Read MUST return a WebsocketCloseError either if a close message is read or if connection
	//    is closed without a close message. In the later case, the 1006 status code MUST be used.
	//    Read MUST drop the existing connection so a new one can be established.
	//
	//	- Read MUST block until a message is read from the server or until connection is closed.
	//
	// # Inputs
	//
	//	- ctx: Context used for tracing purpose
	//
	// # Returns
	//
	//	- MessageType: received message type (Binary | Text)
	//	- []bytes: Message content
	//	- error: in case of connection closure or failure.
	Read(ctx context.Context) (MessageType, []byte, error)
	// # Description
	//
	// Write a single message to the websocket server. Write blocks until message is sent to the
	// server or until an error occurs: context timeout, cancellation, connection closed, ....
	//
	// # Expected behaviour
	//
	//	- Write MUST handle seamlessly message fragmentation, compression and TLS encryption. It is
	//    up to the adapter implementation or to the underlying websocket library to handle these.
	//
	//	- Write MUST NOT handle sending control frames like Close, Ping, etc...
	//
	//	- Write MUST be blocking until a message is sent to the server or until an error occurs.
	//
	// # Inputs
	//
	//	- ctx: Context used for tracing/timeout purpose
	//	- MessageType: received message type (Binary | Text)
	//	- []bytes: Message content
	//
	// # Returns
	//
	//	- error: in case of connection closure, context timeout/cancellation or failure.
	Write(ctx context.Context, msgType MessageType, msg []byte) error
	// # Description
	//
	// Return the underlying websocket connection if any. Returned value has to be type asserted.
	//
	// # Returns
	//
	// The underlying websocket connection if any. Returned value has to be type asserted.
	GetUnderlyingWebsocketConnection() any
}
