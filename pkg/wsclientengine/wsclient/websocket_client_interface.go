package wsclient

import (
	"context"
	"sync"

	adapters "gitlab.com/lake42/go-websocket-client/pkg/wsclientengine/adapters"
)

// Structure which holds data for a websocket close message.
type CloseMessageDetails struct {
	// Close reason code
	CloseReason adapters.StatusCode
	// Close reason message. Can be empty.
	CloseMessage string
}

// Interface which defines callbacks called by the websocket client engine.
type WebsocketClientInterface interface {

	// # Description
	//
	// Callback called when engine has (re)opened a connection to the websocket server. OnOpen is
	// called once, synchronously by the engine during its (re)start phase: no messages or events
	// will be processed until callback completes or a timeout occurs (default: 5 minutes).
	//
	// If OnOpen callback returns an error, websocket engine will:
	//	- If starting: engine will close the opened connection and stop.
	//	- If restarting: engine will close the opened connection and try to restart again.
	//
	// No other callbacks (OnReadError & OnClose) will be used in such cases.
	//
	// During OnOpen call, the provided exit function can be called to definitely stop the engine.
	//
	// # Inputs
	//
	//	- ctx: context produced from the websocket engine context and bound to OnOpen lifecycle.
	//	- conn: Websocket adapter provided during engine creation. Connection is now opened.
	//	- readMutex: A reference to engine read mutex user can lock to pause the engine.
	//	- exit: Function to call to definitely stop the engine (ex: when stuck in retry loop).
	//	- restarting: Flag which indicates whether engine restarts (true) or is starting (false).
	//
	// # Returns
	//
	// nil in case of success or an error if an error occured during OnOpen execution.
	//
	// When engine is restarting, returning an error will cause engine to restart again.
	//
	// # Engine behavior after OnOpen completes
	//
	// If nil is returned and if exit function has not been called, engine will finish starting
	// and create internal goroutines which will manage the websocket connection.
	//
	// If an error is returned, engine will close the opened connection and do the following:
	//		- If engine is starting, engine will definitely stop. Calling exit will do nothing.
	//		- If engine is restarting, engine will try again to restart.
	//		- If engine is restarting and exit has been called, engine will definitely stop.
	OnOpen(
		ctx context.Context,
		conn adapters.WebsocketConnectionAdapterInterface,
		readMutex *sync.Mutex,
		exit context.CancelFunc,
		restarting bool) error

	// # Description
	//
	// Callback called when a message is read from the server. The goroutine which has read the
	// message will block until callback completes. Meanwhile, other goroutines, if any, can read
	// and process other incoming messages unless read mutex is locked.
	//
	// # Inputs
	//
	//	- ctx: context produce from websocket engine context and bound to OnMessage lifecycle.
	//	- conn: Websocket adapter provided during engine creation with a connection opened.
	//	- readMutex: A reference to engine read mutex user can lock to pause the engine.
	//	- restart: Function to call to instruct engine to stop and restart.
	//	- exit: Function to call to definitely stop the engine.
	//	- sessionId: Unique identifier produced by engine for each new websocket connection and
	//    bound to the websocket connection lifetime.
	//	- msgType: Message type returned by read function.
	//	- msg: Received message as a byte array
	//
	// # Engine behavior on exit/restart call
	//
	// 	- No other messages will be read if restart or exit is called.
	//
	//	- Engine will stop after OnMessage is completed: OnClose callback is called and then the
	//    connection is closed. Depending on which function was called, the engine will restart or
	//    stop for good.
	//
	//	- All pending messages will be discarded. The user can continue to read and send messages
	//    in this callback and/or in the OnClose callback until conditions are met to stop the
	//    engine and close the websocket connection.
	OnMessage(
		ctx context.Context,
		conn adapters.WebsocketConnectionAdapterInterface,
		readMutex *sync.Mutex,
		restart context.CancelFunc,
		exit context.CancelFunc,
		sessionId string,
		msgType adapters.MessageType,
		msg []byte)

	// # Description
	//
	// This callback is called each time an error is received when reading messages from the
	// websocket server that is not caused by the connection being closed.
	//
	// The callback is called by the engine goroutine that encountered the error. All engine
	// goroutines will block until the callback is completed. This prevents other messages and
	// events from being processed by the engine while the error is being handled.
	//
	// The engine will restart after OnReadError has finished if one of the following conditions
	// is met:
	// - The websocket connection is closed and the Exit function has not been called.
	// - The restart function has been called.
	//
	// Otherwise, the engine will either continue to process messages on the same connection or
	// shut down if the exit function has been called.
	//
	// Do not close the websocket connection manually: It will be automatically closed if necessary
	// after the OnClose callback has been completed.
	//
	// # Inputs
	//
	//	- ctx: Context produced from the websocket engine context and bound to OnReadError lifecycle.
	//	- conn: Connection to the websocket server.
	//	- readMutex: A reference to engine read mutex user can lock to pause the engine.
	//	- restart: Function to call to instruct engine to stop and restart.
	//	- exit: Function to call to definitely stop the engine.
	//	- err: Error returned by the websocket read operation.
	//
	// # Engine behavior on exit/restart call
	//
	//	- No other messages are read when restart or exit is called.
	//
	//	- Engine will stop after OnReadError: OnClose callback is called and then the connection is
	//    closed. Depending on which function was called, the engine will restart or stop for good.
	//
	//	- All pending messages will be discarded. The user can continue to read and send messages
	//    in this callback and/or in the OnClose callback until conditions are met to stop the
	//    engine and close the websocket connection.
	OnReadError(
		ctx context.Context,
		conn adapters.WebsocketConnectionAdapterInterface,
		readMutex *sync.Mutex,
		restart context.CancelFunc,
		exit context.CancelFunc,
		err error)

	// # Description
	//
	// Callback is called when the websocket connection is closed or about to be closed after a
	// Stop method call or a call to the provided restart/exit functions. Callback is called once
	// by the engine: the engine will not exit or restart until the callback has been completed.
	//
	// Callback can return an optional CloseMessageDetails which will be used to build the close
	// message sent to the server if the connection needs to be closed after OnClose has finished.
	// In such a case, if the returned value is nil, the engine will use 1001 "Going Away" as the
	// close message.
	//
	// Do not close the websocket connection here if it is still open: It will be automatically
	// closed by the engine with a close message.
	//
	// # Inputs
	//
	//	- ctx: Context produced from the websocket engine context and bound to OnClose lifecycle.
	//	- conn: Connection to the websocket server that is closed or about to close.
	//	- readMutex: A reference to engine read mutex user can lock to pause the engine.
	//	- closeMessage: Websocket close message received from server or generated by the engine
	//    when connection has been closed. If nil, connection might not be closed and will be
	//    closed by the engine using the returned close message or the default 1001 "Going Away".
	//
	// # Returns
	//
	// A specific close message to send back to the server if connection has to be closed after
	// this callback completes.
	//
	// # Warning
	//
	// Provided context will already be canceled.
	OnClose(
		ctx context.Context,
		conn adapters.WebsocketConnectionAdapterInterface,
		readMutex *sync.Mutex,
		closeMessage *CloseMessageDetails) *CloseMessageDetails

	// # Description
	//
	// Callback called if an error occurred when the engine called the conn.Close method during
	// the shutdown phase.
	//
	// # Inputs
	//
	//	- ctx:  Context produced OnClose context.
	//	- err: Error returned by conn.Close method
	OnCloseError(
		ctx context.Context,
		err error)

	// # Description
	//
	// Callback called in case an error or a timeout occured when engine tried to restart.
	//
	// # Inputs
	//
	//	- ctx:  Context used for tracing purpose. Will be Done in case a timeout has occured.
	//	- exit: Function to call to stop trying to restart the engine.
	//	- err: Error which has occured when restarting the engine
	//	- retryCount: Number of restart retry since last time engine has successfully (re)started.
	OnRestartError(
		ctx context.Context,
		exit context.CancelFunc,
		err error,
		retryCount int)
}
