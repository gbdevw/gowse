package wsadapters

// Constants used for tracing purpose
const (
	// Instrumentation library package name
	pkgName = "gowsclient.wsadapters"
	// Instrumentation library package version
	pkgVersion = "0.0.0"
	// Namespace used by the spans, attributes and events
	namespace = "websocket"
	// Name of the span used to instrument Dial method call
	spanDial = namespace + "." + "dial"
	// Name of the span used to instrument Close method call
	spanClose = namespace + "." + "close"
	// Name of span used to instrument Ping method call
	spanPing = namespace + "." + "ping"
	// Name of span sed to instrument Write method call
	spanWrite = namespace + "." + "write"
	// Name of span sed to instrument Read method call
	spanRead = namespace + "." + "read"

	// Name of event used when a message has been received
	eventReceived = namespace + "." + "message.received"

	// Name of the attribute used to provide a url
	attrUrl = "url.full"
	// Name of the attribute used to provide a close connection code
	attrCloseCode = namespace + "." + "close.code"
	// Name of the attribute used to provide a close connection reason
	attrCloseReason = namespace + "." + "close.reason"
	// Name of the attribute used to provide the message byte size
	attrMessageByteSize = namespace + "." + "message.size"
	// Name of the attribute used to provide the message frame opcode (from which message type is derived)
	attrMessageType = namespace + "." + "message.opcode"
)
