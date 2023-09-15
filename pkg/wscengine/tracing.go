package wscengine

/*************************************************************************************************/
/* TRACING RELATED CONSTANTS                                                                     */
/*************************************************************************************************/

// Constants used for tracing purpose.
const (
	// Package name used by library tracer
	pkgName = "wscengine"
	// Package version
	pkgVersion = "0.0.0"

	// Namespace used by spans, events and attributes
	namespace = "wscengine"
	// Sub-namespace used by spans related to engine engine backgound tasks
	engineBackgroundNamespace = namespace + ".background"
	// Sub-namespace used by spans related to user provided callbacks
	callbacksNamespace = namespace + ".callback"

	// Name of span used to trace Start public method
	spanEngineStart = namespace + ".start"
	// Name of span used to trace Stop public method
	spanEngineStop = namespace + ".stop"
	// Name of span used to trace startEngine
	spanEngineBackgroundStart = engineBackgroundNamespace + ".start"
	// Name of span used to trace OnOpen callback call
	spanEngineOnOpen = callbacksNamespace + ".on_open"
	// Name of span used to trace engine background tasks which process messages
	spanEngineBackgroundRun = engineBackgroundNamespace + ".run"
	// Name of span used to trace OnReadError callback call
	spanEngineOnReadError = callbacksNamespace + ".on_read_error"
	// Name of span used to trace OnMessage call
	spanEngineOnMessage = callbacksNamespace + ".on_message"
	// Name of span used to trace OnClose call
	spanEngineOnClose = callbacksNamespace + ".on_close"
	// Name of span used to trace OnCloseError call
	spanEngineOnCloseError = callbacksNamespace + ".on_close_error"
	// Name of span used to trace shutdown call
	spanEngineShutdown = engineBackgroundNamespace + ".shutdown"
	// Name of span used to trace restart call
	spanEngineRestart = engineBackgroundNamespace + ".restart"
	// Name of span used to trace OnRestartError callback call
	spanEngineOnRestartError = callbacksNamespace + ".on_restart_error"

	// Event used in span to signal engine goroutine has exited
	eventEngineGoroutineExit = namespace + ".goroutine_exit"
	// Event used in span to signal connection has been closed
	eventConnectionClosed = namespace + ".connection_closed"
	// Event used in span to indicate engine definitely stops
	eventEngineExit = namespace + ".exit"

	// Attribute used to indicate close reason code
	attrCloseCode = namespace + ".close_code"
	// Attribute used to indicate close reason code
	attrCloseReason = namespace + ".close_reason"
	// Attribute used to indicate whether engine is starting or restarting.
	attrRestart = namespace + ".restart"
	// Attribute used to store engine session ID.
	attrSessionId = namespace + ".session_id"
	// Attribute used to store goroutine ID
	attrGoroutineId = namespace + ".goroutine_id"
	// Attribute used to indicate whether connection close should be skipped or not on shutdown
	attrSkipCloseConnection = namespace + ".skip_close_connection"
	// Attribute used to indicate whether a close message was received prior shutdown
	attrHasCloseMessage = namespace + ".has_close_message"
	// Attribute used to indicate received message length
	attrMsgLength = namespace + ".message_length"
	// Attribute used to indicate received message type
	attrMsgType = namespace + ".message_type"
	// Attribute used to indicate whether engine has auto reconnect enabled
	attrAutoReconnect = namespace + ".auto_reconnect"
	// Attribute used to count the number of retries performed
	attrRetryCount = namespace + ".retry_count"
)
