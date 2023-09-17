// The package defines an interface to adapt 3rd parties websocket libraries to websocket engine.
package wsadapters

import (
	"context"
	"net/http"
	"net/url"

	"github.com/stretchr/testify/mock"
)

// Mock for WebsocketConnectionAdapterInterface
type WebsocketConnectionAdapterInterfaceMock struct {
	mock.Mock
}

// Factory
func NewWebsocketConnectionAdapterInterfaceMock() *WebsocketConnectionAdapterInterfaceMock {
	return &WebsocketConnectionAdapterInterfaceMock{
		Mock: mock.Mock{},
	}
}

// # Description
//
// Dial open a connection to the websocket server, performs a WebSocket handshake on url and
// keep internally the underlying websocket connection for further use.
//
// # Expected behaviour
//
//   - Dial MUST block until websocket handshake completes. Websocket handshake and TLS must be
//     handled seamlessly either by the adapter implementation or by the underlying websocket
//     library.
//
//   - Dial MUST NOT return the undelrying websocket connection. The undelrying websocket
//     connection must be kept internally by the adapter implementation in order to be used
//     later by Read, Write, ...
//
//   - Dial SHOULD close any previous opened connection if called again and MUST drop any
//     previous connection. Connection closure must be seamless.
//
// # Inputs
//
//   - ctx: Context used for tracing/timeout purpose
//   - target: Target server URL
//
// # Returns
//
//   - Server response to websocket handshake
//   - error if any
func (mock *WebsocketConnectionAdapterInterfaceMock) Dial(ctx context.Context, target *url.URL) (*http.Response, error) {
	args := mock.Called(ctx, target)
	return args.Get(0).(*http.Response), args.Error(1)
}

// # Description
//
// Send a close message with the provided status code and an optional close reason and close
// the websocket connection.
//
// # Expected behaviour
//
//   - Close MUST be blocking until close message has been sent to the server and until
//     websocket connection closure.
//
//   - There are no guidelines about dropping pending write/read messages when Close is called.
//     Refer to the adapter or the underlying websocket library documentation for guidance.
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
func (mock *WebsocketConnectionAdapterInterfaceMock) Close(ctx context.Context, code StatusCode, reason string) error {
	args := mock.Called(ctx, code, reason)
	return args.Error(0)
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
//   - It can be assumed that there will be at least one concurrent goroutine which continuously
//     call Read method.
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
func (mock *WebsocketConnectionAdapterInterfaceMock) Ping(ctx context.Context) error {
	args := mock.Called(ctx)
	return args.Error(0)
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
func (mock *WebsocketConnectionAdapterInterfaceMock) Read(ctx context.Context) (MessageType, []byte, error) {
	args := mock.Called(ctx)
	return MessageType(args.Int(0)), args.Get(1).([]byte), args.Error(2)
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
func (mock *WebsocketConnectionAdapterInterfaceMock) Write(ctx context.Context, msgType MessageType, msg []byte) error {
	args := mock.Called(ctx, msgType, msg)
	return args.Error(0)
}

// # Description
//
// Return the underlying websocket connection if any. Returned value has to be type asserted.
//
// # Returns
//
// The underlying websocket connection if any. Returned value has to be type asserted.
func (mock *WebsocketConnectionAdapterInterfaceMock) GetUnderlyingWebsocketConnection() any {
	args := mock.Called()
	return args.Get(0)
}
