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

	wsconnadapter "github.com/gbdevw/gowsclient/pkg/wscengine/wsadapters"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
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
	// Tracer
	tracer trace.Tracer
}

// # Description
//
// Factory which creates a new NhooyrWebsocketConnectionAdapter.
//
// # Inputs
//
//   - opts: Optional dial options to use when calling Dial method. Can be nil.
//   - tracerProvider: optional tracer provider to use to get a tracer. If nil, global tracer provider will be used as instead.
//
// # Returns
//
// New NhooyrWebsocketConnectionAdapter
func NewNhooyrWebsocketConnectionAdapter(opts *websocket.DialOptions, tracerProvider trace.TracerProvider) *NhooyrWebsocketConnectionAdapter {
	if tracerProvider == nil {
		// Get global tracer provider if none is provided
		tracerProvider = otel.GetTracerProvider()
	}
	return &NhooyrWebsocketConnectionAdapter{
		conn:   nil,
		opts:   opts,
		mu:     sync.Mutex{},
		tracer: tracerProvider.Tracer(packageName, trace.WithInstrumentationVersion(packageVersion)),
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
//   - Dial MUST return an error if called while there is already an alive connection. It is up
//     to the adapter implementation or to the underlying websocket library to detect whether a
//     connection is already alive or not.
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
func (adapter *NhooyrWebsocketConnectionAdapter) Dial(ctx context.Context, target *url.URL) (*http.Response, error) {
	select {
	case <-ctx.Done():
		// Shortcut if context is done (timeout/cancel)
		return nil, ctx.Err()
	default:
		// Lock internal mutex before accessing internal state
		adapter.mu.Lock()
		defer adapter.mu.Unlock()
		// Start Dial span
		ctx, span := adapter.tracer.Start(ctx, dialSpan, trace.WithSpanKind(trace.SpanKindClient), trace.WithAttributes(
			attribute.String(dialSpanUrlAttr, target.String()),
		))
		defer span.End()
		// Check whether there is already a connection set
		if adapter.conn != nil {
			// Start span for close
			code := websocket.StatusGoingAway
			_, spanClose := adapter.tracer.Start(ctx, dialCloseSpan, trace.WithSpanKind(trace.SpanKindClient), trace.WithAttributes(
				attribute.Int(dialCloseSpanCodeAttr, int(code)),
			))
			// Close and drop previous connection
			err := adapter.conn.Close(code, "")
			adapter.conn = nil
			// Record error if any and close span
			spanClose.RecordError(err)
			spanClose.SetStatus(codes.Ok, codes.Ok.String())
			spanClose.End()
		}
		// Open websocket connection
		conn, res, err := websocket.Dial(ctx, target.String(), adapter.opts)
		if err != nil {
			// Trace and return error
			span.RecordError(err)
			span.SetStatus(codes.Error, codes.Error.String())
			return nil, err
		}
		// Persist connection internally and return
		adapter.conn = conn
		span.SetStatus(codes.Ok, codes.Ok.String())
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
//   - Close MUST close the connection even if provided context is already canceled.
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
	// Start span
	_, span := adapter.tracer.Start(ctx, closeSpan, trace.WithSpanKind(trace.SpanKindClient), trace.WithAttributes(
		attribute.Int(closeSpanCodeAttr, int(code)),
	))
	defer span.End()
	// Check whether there is already a connection set
	if adapter.conn != nil {
		err := adapter.conn.Close(convertToNhooyrStatusCodes(code), reason)
		// Void connection
		adapter.conn = nil
		// Check error
		if err != nil {
			if err.Error() == "failed to close WebSocket: already wrote close" {
				// Change to an error which wraps a net.ErrClosed
				err = fmt.Errorf("failed to close WebSocket: %w", net.ErrClosed)
			}
			// Trace error
			span.RecordError(err)
			span.SetStatus(codes.Error, codes.Error.String())
			return err
		} else {
			span.SetStatus(codes.Ok, codes.Ok.String())
			return nil
		}
	} else {
		// No connection is set -> trace & return error
		err := fmt.Errorf("close failed because no connection is already up")
		span.RecordError(err)
		span.SetStatus(codes.Error, codes.Error.String())
		return err
	}
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
		// Start span
		ctx, span := adapter.tracer.Start(ctx, pingSpan, trace.WithSpanKind(trace.SpanKindClient))
		defer span.End()
		// Check whether there is already a connection set
		if conn != nil {
			err := conn.Ping(ctx)
			if err != nil {
				// Trace error and return
				span.RecordError(err)
				span.SetStatus(codes.Ok, codes.Ok.String())
				return err
			} else {
				// Ok
				span.SetStatus(codes.Ok, codes.Ok.String())
				return nil
			}
		} else {
			// No connection is set -> trace & return error
			err := fmt.Errorf("ping failed because no connection is already up")
			span.RecordError(err)
			span.SetStatus(codes.Error, codes.Error.String())
			return err
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
//   - MessageType: received message type. -1 in case of error.
//   - []bytes: Message content. Nil in case of error.
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
		// Start span
		ctx, span := adapter.tracer.Start(ctx, readSpan, trace.WithSpanKind(trace.SpanKindClient))
		defer span.End()
		// Check whether there is already a connection set
		if conn != nil {
			nhooyrMsgType, msg, err := conn.Read(ctx)
			if err != nil {
				// Trace error
				span.RecordError(err)
				span.SetStatus(codes.Error, codes.Error.String())
				// First, check context to know if error was caused by timeout/cancel
				select {
				case <-ctx.Done():
					// Return context error
					return -1, nil, err
				default:
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
			} else {
				// Convert to wsclient msgtype
				msgType := convertFromNhooyrMsgTypes(nhooyrMsgType)
				msgTypeAttrVal := "binary"
				if msgType == wsconnadapter.Text {
					msgTypeAttrVal = "text"
				}
				// Trace
				span.AddEvent(rcvdEvent, trace.WithAttributes(
					attribute.Int(writeSpanBytesAttr, len(msg)),
					attribute.String(writeSpanTypeAttr, msgTypeAttrVal),
				))
				span.SetStatus(codes.Ok, codes.Ok.String())
				// Return result
				return msgType, msg, nil
			}
		} else {
			// No connection is set -> trace & return error
			err := fmt.Errorf("read failed because no connection is already up")
			span.RecordError(err)
			span.SetStatus(codes.Error, codes.Error.String())
			return -1, nil, err
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
		// Start span
		msgTypeAttrVal := "binary"
		if msgType == wsconnadapter.Text {
			msgTypeAttrVal = "text"
		}
		ctx, span := adapter.tracer.Start(ctx, writeSpan, trace.WithSpanKind(trace.SpanKindClient), trace.WithAttributes(
			attribute.Int(writeSpanBytesAttr, len(msg)),
			attribute.String(writeSpanTypeAttr, msgTypeAttrVal),
		))
		defer span.End()
		// Check whether there is already a connection set
		if conn != nil {
			err := conn.Write(ctx, convertToNhooyrMsgTypes(msgType), msg)
			if err != nil {
				// Trace error and return
				span.RecordError(err)
				span.SetStatus(codes.Ok, codes.Ok.String())
				return err
			} else {
				// Ok
				span.SetStatus(codes.Ok, codes.Ok.String())
				return nil
			}
		} else {
			// No connection is set -> trace & return error
			err := fmt.Errorf("write failed because no connection is already up")
			span.RecordError(err)
			span.SetStatus(codes.Error, codes.Error.String())
			return err
		}
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
