// The package defines an interface to adapt 3rd parties websocket libraries to websocket engine.
package adapters

/*************************************************************************************************/
/* WEBSOCKET RELATED CONSTANTS                                                                   */
/*************************************************************************************************/

// Constants for RFC6455 defined close status codes
//
// RFC: https://www.rfc-editor.org/rfc/rfc6455.html#section-7.4.1
//
// Code names are inspired by: https://www.iana.org/assignments/websocket/websocket.xhtml
type StatusCode int

const (
	// 1000 indicates a normal closure, meaning that the purpose for
	// which the connection was established has been fulfilled.
	NormalClosure StatusCode = iota + 1000
	// 1001 indicates that an endpoint is "going away", such as a server
	// going down or a browser having navigated away from a page.
	GoingAway
	// 1002 indicates that an endpoint is terminating the connection due
	// to a protocol error.
	ProtocolError
	// 1003 indicates that an endpoint is terminating the connection
	// because it has received a type of data it cannot accept (e.g., an
	// endpoint that understands only text data MAY send this if it
	// receives a binary message).
	UnsupportedData
	// 1005 is a reserved value and MUST NOT be set as a status code in a
	// Close control frame by an endpoint.  It is designated for use in
	// applications expecting a status code to indicate that no status
	// code was actually present.
	NoStatusReceived StatusCode = iota + 1000 + 1 // Skip 1003
	// 1006 is a reserved value and MUST NOT be set as a status code in a
	// Close control frame by an endpoint.  It is designated for use in
	// applications expecting a status code to indicate that the
	// connection was closed abnormally, e.g., without sending or
	// receiving a Close control frame.
	AbnormalClosure
	// 1007 indicates that an endpoint is terminating the connection
	// because it has received data within a message that was not
	// consistent with the type of the message (e.g., non-UTF-8 [RFC3629]
	// data within a text message).
	InvalidFramePayloadData
	// 1008 indicates that an endpoint is terminating the connection
	// because it has received a message that violates its policy.  This
	// is a generic status code that can be returned when there is no
	// other more suitable status code (e.g., 1003 or 1009) or if there
	// is a need to hide specific details about the policy.
	PolicyViolation
	// 1009 indicates that an endpoint is terminating the connection
	// because it has received a message that is too big for it to
	// process.
	MessageTooBig
	// 1010 indicates that an endpoint (client) is terminating the
	// connection because it has expected the server to negotiate one or
	// more extension, but the server didn't return them in the response
	// message of the WebSocket handshake.  The list of extensions that
	// are needed SHOULD appear in the /reason/ part of the Close frame.
	// Note that this status code is not used by the server, because it
	// can fail the WebSocket handshake instead.
	MandatoryExtension
	// 1011 indicates that a server is terminating the connection because
	// it encountered an unexpected condition that prevented it from
	// fulfilling the request.
	InternalError
	// 1015 is a reserved value and MUST NOT be set as a status code in a
	// Close control frame by an endpoint.  It is designated for use in
	// applications expecting a status code to indicate that the
	// connection was closed due to a failure to perform a TLS handshake
	// (e.g., the server certificate can't be verified).
	TLSHandshake StatusCode = iota + 1000 + 3 // Skip 1012 to 1014
)

// Websocket message types which can be received.
//
// Codes mimics RFC6455 frame opcodes. Control frames like continuation, close, ping, pong and
// others are excluded as the library focuses on message level and not frame level. Furthermore,
// the underlying websocket library used by the engine is expected to seamlessly handle message
// fragmentation and control frames like close, ping & pong.
//
// https://datatracker.ietf.org/doc/html/rfc6455#section-5.6
type MessageType int

const (
	// Denotes a text message
	Text MessageType = iota + 1
	// Denotes a binary message
	Binary
)
