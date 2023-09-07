package wsclientengine

/*************************************************************************************************/
/* TRACING RELATED CONSTANTS                                                                     */
/*************************************************************************************************/

// Constants used for tracing purpose.
const (
	// Package name used by lib. tracer
	pkgName = "wscengine"
	// Package version
	pkgVersion = "0.0.0"

	// Namespace used by spans, events and attributes
	namespace = "wscengine"
	// Subnamespace used by spans related to engine private methods
	engineInternalRoot = namespace + ".internal"

	// Name of span used to trace Start
	spanEngineStart = namespace + ".start"
	// Name of span used to trace Stop
	spanEngineStop = namespace + ".stop"
	// Name of span used to trace startEngine
	spanEngineInternalStart = engineInternalRoot + ".start"
	// Name of span used to trace OnOpen call
	spanEngineOnOpen = spanEngineInternalStart + ".on_open"
	// Name of span used to trace engine internal goroutine tasks
	spanEngineInternalRun = engineInternalRoot + ".run"
	// Name of span used to trace OnError call
	spanEngineOnReadError = spanEngineInternalRun + ".on_read_error"
	// Name of span used to trace OnMessage call
	spanEngineOnMessage = spanEngineInternalRun + ".on_message"
	// Name of span used to trace OnClose call
	spanEngineOnClose = spanEngineInternalRun + ".on_close"
	// Name of span used to trace OnCloseError call
	spanEngineOnCloseError = spanEngineInternalRun + ".on_close_error"
	// Name of span used to trace shutdown call
	spanEngineShutdown = spanEngineInternalRun + ".shutdown"
	// Name of span used to trace restart call
	spanEngineRestart = engineInternalRoot + ".restart"
	// Name of span used to trace OnRestartError call
	spanEngineOnRestartError = spanEngineRestart + ".on_restart_error"

	// Event used in span to signal engine goroutine has exited
	eventEngineGoroutineExit = spanEngineInternalRun + ".goroutine.exit"
	// Event used in span to signal connection has been closed
	eventConnectionClosed = namespace + ".connection_closed"
	// Event used in span to indicate engine definitely stops
	eventEngineExit = namespace + ".exit"

	// Attribute used to indicate close reason code
	attrCloseCode = namespace + ".close.code"
	// Attribute used to indicate close reason code
	attrCloseReason = namespace + ".close.reason"
	// Attribute used to indicate whether engine is starting or restarting.
	attrRestart = namespace + ".restart"
	// Attribute used to store engine session ID.
	attrSessionId = namespace + ".session.id"
	// Attribute used to store goroutine ID
	attrGoroutineId = namespace + ".goroutine.id"
	// Attribute used to indicate whether connection close should be skipped or not on shutdown
	attrSkipCloseConnection = namespace + ".skip_close_connection"
	// Attribute used to indicate whether a close message was received prior shutdown
	attrHasCloseMessage = namespace + ".has_close_message"
	// Attribute used to indicate received message length
	attrMsgLength = namespace + ".message.length"
	// Attribute used to indicate received message type
	attrMsgType = namespace + ".message.type"
	// Attribute used to indicate whether engine has auto reconnect enabled
	attrAutoReconnect = namespace + ".auto_reconnect"
	// Attribute used to count the number of retries performed
	attrRetryCount = namespace + ".retry.count"
)
