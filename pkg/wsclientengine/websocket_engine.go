package wsclientengine

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/url"
	"sync"
	"time"

	"github.com/google/uuid"
	adapters "gitlab.com/lake42/go-websocket-client/pkg/wsclientengine/adapters"
	"gitlab.com/lake42/go-websocket-client/pkg/wsclientengine/wsclient"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// Engine which manages a websocket connection, read incoming messages, calls appropriate client
// callbacks and automatically reopen a connection with the server if connection is interrupted.
type WebsocketEngine struct {
	// Context built from context provided as Start input and used by websocket engine for tracing
	// and purposes. All subcontextes forwarded to user callbacks are built from this context.
	engineCtx context.Context
	// Cancel function associated to engineCtx and used to stop the engine.
	engineStopFunc context.CancelFunc
	// Target websocket server URL.
	target *url.URL
	// Websocket connection adapter used by engine to establish and use the websocket connection.
	conn adapters.WebsocketConnectionAdapterInterface
	// User defined callbacks called by the websocket engine.
	wsclient wsclient.WebsocketClientInterface
	// Configuration options used by the engine.
	engineCfgOpts *WebsocketEngineConfigurationOptions
	// Tracer used to instrument websocket engine code.
	tracer trace.Tracer
	// Internal state flag used to know if the engine has started or not.
	started bool
	// Internal channel used to signal engine has finished stopping.
	stoppedChannel chan bool
	// Internal mutex used to protect start/stop methods.
	startMutex *sync.Mutex
	// Mutex used to pause the engine and prevent it from processing messages. Mutex can be locked
	// to temporarely pause the engine and pilot the underlying websocket connection.
	readMutex *sync.Mutex
}

// # Description
//
// Factory - Return a new, not started websocket engine.
//
// # Inputs
//   - url: Target websocket server URL.
//   - conn: Websocket connection adapter engine will use to connect to the target server.
//   - wsclient: User provided callbacks which will be called by the websocket engine.
//   - opts: Engine configuration options. If nil, default options are used.
//   - traceProvider: OpenTelemetry tracer provider to use. If nil, global TracerProvider is used.
//
// # Return
//
// Factory returns a new, non-started websocket engine in case of success. If provided options
// are invalid, factory will return nil and an error.
func NewWebsocketEngine(
	url *url.URL,
	conn adapters.WebsocketConnectionAdapterInterface,
	wsclient wsclient.WebsocketClientInterface,
	opts *WebsocketEngineConfigurationOptions,
	tracerProvider trace.TracerProvider) (*WebsocketEngine, error) {

	// Check provided URL is not nil
	if url == nil {
		return nil, fmt.Errorf("provided url is nil")
	}
	// Check provided connection adapter is not nil
	if conn == nil {
		return nil, fmt.Errorf("provided connection adapter is nil")
	}
	// Check provided websocket client is not nil
	if wsclient == nil {
		return nil, fmt.Errorf("provided websocket client is nil")
	}
	// Use default options if not set
	if opts == nil {
		opts = NewWebsocketEngineConfigurationOptions()
	}
	// Validate options
	err := Validate(opts)
	if err != nil {
		return nil, err
	}
	// Get tracer provider from global tracer provider if not provided
	if tracerProvider == nil {
		tracerProvider = otel.GetTracerProvider()
	}
	// Return websocket engine
	return &WebsocketEngine{
		engineCtx: nil,
		engineStopFunc: func() {
		},
		target:         url,
		conn:           conn,
		wsclient:       wsclient,
		engineCfgOpts:  opts,
		tracer:         tracerProvider.Tracer(pkgName, trace.WithInstrumentationVersion(pkgVersion)),
		started:        false,
		stoppedChannel: make(chan bool, 1),
		startMutex:     &sync.Mutex{},
		readMutex:      &sync.Mutex{},
	}, nil
}

// # Description
//
// Start the websocket engine that will connect to the server, call OnOpen callback and then spawn
// goroutines which will continuously fetch messages and call appropriate user defined callbacks.
//
// The Start method blocks until:
//   - Engine startup phase completes.
//   - The engine returns an error from its start phase.
//   - A OnOpenTimeout occurs (if enabled).
//
// # Inputs
//
//   - ctx: context used as parent of all engine contextes. Used for tracing/coordination purpose.
//
// # Return
//
// The method will return nil on success.
//
// The method returns an error if:
//   - Provided context is canceled.
//   - Engine has already started.
//   - OnOpen returned an error: In this case, returned error embed error returned by OnOpen.
//   - The engine fails to open a connection to the websocket server.
//   - A timeout occured during startup phase.
//
// # What happen to the opened websocket connection if an error occurs during startup phase?
//
// If an error occurs during the startup phase (timeout, context canceld, ...), the engine will try
// to close the websocket connection by sending a "Going away" 1001 close message.
//
// # What to do when an error occurs?
//
// Some errors are not definitive failures:
//   - The engine fails to open a connection to the websocket server.
//   - OnOpen callback returned an error.
//   - A timeout occured during startup phase
//
// When an error occurs when engine is starting for the first time, the engine will not retry: it is
// up to the user code to try again calling Start().
func (wsengine *WebsocketEngine) Start(ctx context.Context) error {
	// Create websocket engine context & cancel function from context
	wsengine.engineCtx, wsengine.engineStopFunc = context.WithCancel(ctx)
	// Create span
	ctx, span := wsengine.tracer.Start(wsengine.engineCtx, spanEngineStart,
		trace.WithSpanKind(trace.SpanKindInternal))
	defer span.End()
	// Check provided context
	select {
	case <-ctx.Done():
		// Shortcut Start as user provided context is already canceled
		err := EngineStartError{Err: ctx.Err()}
		span.RecordError(err)
		span.SetStatus(codes.Error, codes.Error.String())
		return err
	default:
		// If enabled, create a subcontext with timeout for start operation
		if wsengine.engineCfgOpts.OnOpenTimeoutMs > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(
				wsengine.engineCtx,
				time.Duration(wsengine.engineCfgOpts.OnOpenTimeoutMs*int64(time.Millisecond)))
			defer cancel()
		}
		// Create internal channel to wait for the engine start completion signal
		startupChannel := make(chan error, 1)
		// Start a goroutine that will kick off the websocket engine.
		go wsengine.startEngine(ctx, false, startupChannel, wsengine.engineStopFunc)
		// Read from error channel or context done channel to know when the engine has finished
		// starting or if a timeout has occured
		select {
		case err := <-startupChannel:
			// Engine has finished starting and sent back either a nil value (OK) or an error.
			if err != nil {
				// An error occured while engine was starting
				span.RecordError(err) // Noop if err is nil
				span.SetStatus(codes.Error, codes.Error.String())
				return err
			} else {
				// Engine has started
				span.SetStatus(codes.Ok, codes.Ok.String())
				return nil
			}
		case <-ctx.Done():
			// A timeout has occured or provided parent context has been canceled.
			span.RecordError(ctx.Err())
			span.SetStatus(codes.Error, codes.Error.String())
			return EngineStartError{Err: ctx.Err()}
		}
	}
}

// # Description
//
// Definitely stop the websocket engine. The method will block until the engine has stopped. The
// engine will call OnClose callback, close the websocket connection and exit.
//
// # Return
//
// The method returns nil on success or an error if:
//   - the websocket engine is not started.
//   - a timeout has occured while waiting for the engine to stop (if enabled).
//
// # Warning - Unlock read mutex before calling Stop()
//
// If you have locked the read Mutex, the engine will not stop and signal it has stopped until read
// Mutex has been released. As Stop blocks until stop signal is emitted by the engine, the calling
// goroutine will be blocked on Stop until the read mutex is unlocked by another goroutine.
//
// There is simple way to prevent this issue from occuring: Unlock read mutex before calling Stop!
func (wsengine *WebsocketEngine) Stop(ctx context.Context) error {
	// Lock start mutex
	wsengine.startMutex.Lock()
	defer wsengine.startMutex.Unlock()
	// Start stop span
	ctx, span := wsengine.tracer.Start(ctx, spanEngineStop,
		trace.WithSpanKind(trace.SpanKindInternal))
	defer span.End()
	// Check if engine is started
	if wsengine.started {
		// Call engine context cancel function to start stopping the engine. The engine goroutines
		// will all exit except one which will perform the engine shutdown by calling user OnClose
		// callback, close the websocket connection and exit.
		wsengine.engineStopFunc()
		// If enabled, create subcontext with timeout for stop operation
		if wsengine.engineCfgOpts.StopTimeoutMs > 0 {
			var stopCancel func()
			ctx, stopCancel = context.WithTimeout(
				ctx,
				time.Duration(wsengine.engineCfgOpts.StopTimeoutMs*int64(time.Millisecond)))
			defer stopCancel()
		}
		// Wait engine stop completes or times out
		select {
		case <-ctx.Done():
			// Trace & return error: time out or context canceled
			span.RecordError(ctx.Err())
			span.SetStatus(codes.Error, codes.Error.String())
			wsengine.started = false
			return ctx.Err()
		case <-wsengine.stoppedChannel:
			// Mark engine as stopped
			span.SetStatus(codes.Ok, codes.Ok.String())
			wsengine.started = false
			return nil
		}
	} else {
		// Trace & return error: engine is not started
		err := fmt.Errorf("websocket engine is not started")
		span.RecordError(err)
		span.SetStatus(codes.Error, codes.Error.String())
		return err
	}
}

/*************************************************************************************************/
/* UTILS                                                                                         */
/*************************************************************************************************/

// # Description
//
// Returns the websocket engine started state. If the engine is starting or stopping, the method
// will block until engine has finished starting or stopping amd then will return the engine state.
func (wsengine *WebsocketEngine) IsStarted() bool {
	// Lock start mutex and then return started state.
	wsengine.startMutex.Lock()
	defer wsengine.startMutex.Unlock()
	return wsengine.started
}

// # Description
//
// Get read Mutex used to pause the engine and prevent it from processing messages and events. User
// can can lock it to temporarely take full control over the underlying websocket connection.
//
// An example of such case is when the user wants to handle a synchronous request-response pattern:
// User can lock the mutex (from inside a callback or outside of it), send a message to the server
// and then process incoming messages while waiting for a specific reply or error from the server.
//
// Once user has finished done, user has to release the mutex to 'resume' the engine.
//
// # Warning
//
// Be sure to unlock the mutex when using it! Engine goroutines will be blocked and engine will
// not process any message and will not stop until read mutex is unlocked.
//
// # Return
//
// The mutex used to pause the websocket engine.
func (wsengine *WebsocketEngine) GetReadMutex() *sync.Mutex {
	return wsengine.readMutex
}

/*************************************************************************************************/
/* WEBSOCKET ENGINE                                                                              */
/*************************************************************************************************/

// # Description
//
// Method will first check whether the engine is not started or is restarting. Then, it will open
// a websocket connection to the server using the internal websocket connection adapter and call
// OnOpen callback once connection is established.
//
// If engine is already started and not restarting or if provided context is canceled, method must
// publish an EngineStartError embedding the appropriate error to startupChannel.
//
// If method fails to open a connection with the adapter, method must publish the returned error
// to the provided error channel and exit.
//
// If OnOpen returns an error method must publish the error to the provided error channel, close
// the opened websocket connection and exit.
//
// If one of the restart or exit functions have been called in OnOpen callback, method must publish
// the context error to the provided error channeel, close the opened connection and exit.
//
// Once the OnOpen callback has completed sucessfully, method will create engine goroutines that
// will read messages from websocket connection and call appropriate user callbacks. Then, method
// will publish a nil value to the provided error channel to signal engine has finished starting
// and will exit.
//
// # Inputs
//
//   - ctx: Context used for tracing/coordination purposes
//   - startupChannel: Channel used by the engine to signal it has finished starting.
//   - restart: Indicates if the method is called because the engine starts or is restarting
//   - exit: Function to call to prevent the engine from restarting
func (wsengine *WebsocketEngine) startEngine(
	ctx context.Context,
	restart bool,
	startupChannel chan error,
	exit context.CancelFunc) {

	// Lock start mutex
	wsengine.startMutex.Lock()
	defer wsengine.startMutex.Unlock()
	// Create span
	ctx, span := wsengine.tracer.Start(ctx, spanEngineBackgroundStart,
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.Bool(attrRestart, restart),
		))
	defer span.End()
	// Check provided context is not canceled
	select {
	case <-ctx.Done():
		// Shortcut as context has been canceled
		wsengine.started = false // Set started flag to false
		err := EngineStartError{Err: ctx.Err()}
		span.RecordError(err)
		span.SetStatus(codes.Error, codes.Error.String())
		// Channel back error before exiting
		startupChannel <- err
	default:
		// Check if engine is not started or is restarting
		if !wsengine.started || restart {
			// Open websocket connection to the target server
			_, err := wsengine.conn.Dial(ctx, wsengine.target)
			// Check channel done to detect timeout
			select {
			case <-ctx.Done():
				// By safety, force close the connection but do not handle error if any
				rsn := "websocket client has been interrupted"
				rsnCode := adapters.GoingAway
				span.AddEvent(eventConnectionClosed, trace.WithAttributes(
					attribute.Int(attrCloseCode, int(rsnCode)),
					attribute.String(attrCloseReason, rsn),
				))
				err := wsengine.conn.Close(ctx, rsnCode, rsn)
				span.RecordError(err)
				// Trace, channel back error and exit
				err = EngineStartError{Err: ctx.Err()}
				span.RecordError(err)
				span.SetStatus(codes.Error, codes.Error.String())
				startupChannel <- err
				return
			default:
				if err != nil {
					// Trace, channel back error and exit
					err = EngineStartError{Err: err}
					span.RecordError(err)
					span.SetStatus(codes.Error, codes.Error.String())
					startupChannel <- err
					return
				}
				// Start a span to trace OnOpen callback
				onOpenCtx, onOpenSpan := wsengine.tracer.Start(ctx, spanEngineOnOpen,
					trace.WithSpanKind(trace.SpanKindInternal))
				// Call OnOpen callback
				err = wsengine.wsclient.OnOpen(
					onOpenCtx,
					wsengine.conn,
					wsengine.readMutex,
					exit,
					restart)
				if err != nil {
					// Stop onOpenSpan with an error status
					onOpenSpan.RecordError(err)
					onOpenSpan.SetStatus(codes.Error, codes.Error.String())
					onOpenSpan.End()
					// Close websocket connection withtout calling callbacks
					rsn := "websocket client failed to start"
					rsnCode := adapters.GoingAway
					span.AddEvent(eventConnectionClosed, trace.WithAttributes(
						attribute.Int(attrCloseCode, int(rsnCode)),
						attribute.String(attrCloseReason, rsn),
					))
					errClose := wsengine.conn.Close(ctx, rsnCode, rsn)
					span.RecordError(errClose)
					// Trace & channel back error: error returned by onOpen
					err = EngineStartError{Err: err}
					span.SetStatus(codes.Error, codes.Error.String())
					startupChannel <- err
					return
				} else {
					// Stop onOpenSpan with a OK status
					onOpenSpan.SetStatus(codes.Ok, codes.Ok.String())
					onOpenSpan.End()
					// Startup finished - Create a session context from start context
					sessionCtx, sessionCancelFunc := context.WithCancel(wsengine.engineCtx)
					// Create a monitor all goroutines will share to ensure shutdown is called once
					shutdownSync := &sync.Once{}
					// Create a UUID which will identify the session
					sessionUuid := uuid.New()
					// Start the first goroutine which will run the engine.
					// Used to prevent compiler warning -> cancelFunc not used on all paths
					go wsengine.runEngine(
						sessionCtx,
						sessionCancelFunc,
						exit,
						wsengine.conn,
						shutdownSync,
						sessionUuid.String(),
						uuid.New().String(),
					)
					// Start additional go routines that will run the engine
					for i := 1; i < wsengine.engineCfgOpts.ReaderRoutinesCount; i++ {
						go wsengine.runEngine(
							sessionCtx,
							sessionCancelFunc,
							exit,
							wsengine.conn,
							shutdownSync,
							sessionUuid.String(),
							uuid.New().String(),
						)
					}
					// Channel back nil (success) and exit
					wsengine.started = true
					span.SetStatus(codes.Ok, codes.Ok.String())
					startupChannel <- nil
				}
			}
		} else {
			// Trace & channel back error: engine has already started
			err := EngineStartError{
				Err: fmt.Errorf("engine has already started"),
			}
			span.RecordError(err)
			span.SetStatus(codes.Error, codes.Error.String())
			startupChannel <- err
		}
	}
}

// # Description
//
// Engine internal goroutine task. The function will continuously lock read mutex, watch if session
// context is not canceled and then call conn.Read to wait for a new message.
//
// When conn.Read completes, goroutine will first check whether session context has been canceled.
// In such case, goroutine will perform engine shutdown (call OnClose callback + close connection
// + trigger engine restart if appplicable), release read mutex and exit.
//
// If session has not been canceled, goroutine will check whether conn.Read has returned an error.
// In such case, there are two different possibilities:
//   - Error is a WebsocketCloseError which means connection has been closed: Goroutine will then
//     cancel session context (so other goroutines will exit once read mutex is released), perform
//     engine shutdown, release read mutex and exit.
//   - Error is not a WebsocketCloseError. Goroutine will call OnReadError and provide the error. When
//     OnReadError completes, gouroutine will check the session context again: if session context has
//     been canceled during OnReadError call (by user), goroutine will shutdown the engine, realease
//     read mutex and exit. If session context has not been canceled during OnReadError call, goroutine
//     will release read mutex and loop.
//
// Finally, in case conn.Read returns a message, no error has occured and session context has not
// been canceled, goroutine will release read mutex and call OnMessage callback to process the
// received message. Once OnMessage callback completes, goroutine will loop.
//
// # Inputs
//
//   - sessionCtx: Context produced from engine context and bound to websocket connection lifetime.
//   - cancelSession: Function to call to cancel session context and stop all other goroutines.
//   - exit: Same effect as cancelSession plus engine definitely stop.
//   - conn: Websocket connection adapter used to read messages and manage connection.
//   - shutdownSync: Object used to ensure engine shutdown is performed exactly once.
//   - sessionId: Id bound to the connection lifecycle. Used to correlate traces.
//   - routineId: Unique ID bound to the goroutine lifetime.
func (wsengine *WebsocketEngine) runEngine(
	sessionCtx context.Context,
	cancelSession context.CancelFunc,
	exit context.CancelFunc,
	conn adapters.WebsocketConnectionAdapterInterface,
	shutdownSync *sync.Once,
	sessionId string,
	routineId string) {

	// Run continuously until exit
	for {
		// Lock read mutex
		wsengine.readMutex.Lock()
		// Start span
		ctx, span := wsengine.tracer.Start(sessionCtx, spanEngineBackgroundRun,
			trace.WithSpanKind(trace.SpanKindInternal),
			trace.WithAttributes(
				attribute.String(attrSessionId, sessionId),
				attribute.String(attrGoroutineId, routineId),
			))
		select {
		case <-sessionCtx.Done():
			// Websocket session has been canceled. Call once shutdownEngine.
			shutdownSync.Do(func() { wsengine.shutdownEngine(ctx, nil, false) })
			// Unlock read mutex - All other engine goroutines will exit
			wsengine.readMutex.Unlock()
			// Exit
			span.AddEvent(eventEngineGoroutineExit, trace.WithAttributes(
				attribute.String(attrSessionId, sessionId),
				attribute.String(attrGoroutineId, routineId),
			))
			span.RecordError(sessionCtx.Err())
			span.SetStatus(codes.Ok, codes.Ok.String())
			span.End()
			return
		default:
			// Read message
			msgType, msg, err := wsengine.conn.Read(sessionCtx)
			select {
			case <-sessionCtx.Done():
				// Session has been canceled - Call once shutdownEngine
				shutdownSync.Do(func() { wsengine.shutdownEngine(ctx, nil, false) })
				// Unlock read mutex - All other engine goroutines will exit
				wsengine.readMutex.Unlock()
				// Exit
				span.AddEvent(eventEngineGoroutineExit, trace.WithAttributes(
					attribute.String(attrSessionId, sessionId),
					attribute.String(attrGoroutineId, routineId),
				))
				span.RecordError(sessionCtx.Err())
				span.SetStatus(codes.Ok, codes.Ok.String())
				span.End()
				return
			default:
				// conn.Read returned an error
				if err != nil {
					// Record received error in span
					span.RecordError(err)
					// Check whether error is a WebsocketCloseError
					closeErr := new(adapters.WebsocketCloseError)
					if errors.As(err, closeErr) {
						// Add connection closed event to span
						span.AddEvent(eventConnectionClosed, trace.WithAttributes(
							attribute.String(attrCloseReason, closeErr.Reason),
							attribute.Int(attrCloseCode, int(closeErr.Code)),
						))
						// Cancel session context to force other goroutines to exit
						cancelSession()
						// Craft close message from close error data
						closeMsg := &wsclient.CloseMessageDetails{
							CloseReason:  closeErr.Code,
							CloseMessage: closeErr.Reason,
						}
						// Shutdown the engine - skip websocket connection close
						shutdownSync.Do(func() { wsengine.shutdownEngine(ctx, closeMsg, true) })
						// Unnlock mutex - All other engine goroutines will exit
						wsengine.readMutex.Unlock()
						// Exit
						span.AddEvent(eventEngineGoroutineExit, trace.WithAttributes(
							attribute.String(attrSessionId, sessionId),
							attribute.String(attrGoroutineId, routineId),
						))
						span.End()
						span.SetStatus(codes.Ok, codes.Ok.String())
						return
					} else {
						// An error occured - call OnReadError callback
						onErrorctx, onErrorSpan := wsengine.tracer.Start(ctx, spanEngineOnReadError,
							trace.WithSpanKind(trace.SpanKindInternal))
						wsengine.wsclient.OnReadError(
							onErrorctx,
							conn,
							wsengine.readMutex,
							cancelSession,
							exit,
							err)
						onErrorSpan.SetStatus(codes.Ok, codes.Ok.String())
						onErrorSpan.End()
						// Check context to determine if engine has to be restarted/stopped
						select {
						case <-sessionCtx.Done():
							// Shutdown the engine - do not skip websocket connection close
							shutdownSync.Do(func() { wsengine.shutdownEngine(ctx, nil, false) })
							// Unnlock mutex - All other engine goroutines will exit
							wsengine.readMutex.Unlock()
							// Exit
							span.AddEvent(eventEngineGoroutineExit, trace.WithAttributes(
								attribute.String(attrSessionId, sessionId),
								attribute.String(attrGoroutineId, routineId),
							))
							span.End()
							span.SetStatus(codes.Ok, codes.Ok.String())
							return
						default:
							// Unlock read mutex & loop
							wsengine.readMutex.Unlock()
							span.End()
							span.SetStatus(codes.Ok, codes.Ok.String())
						}
					}
				} else {
					// We have a message to process -> release mutex first to allow other routines
					// to process new messages while goroutine process this one.
					wsengine.readMutex.Unlock()
					// Start span for OnMessage calllback call
					msgTypeTxt := "binary"
					if msgType == adapters.Text {
						msgTypeTxt = "text"
					}
					onMessagectx, onMessageSpan := wsengine.tracer.Start(ctx, spanEngineOnMessage,
						trace.WithSpanKind(trace.SpanKindInternal),
						trace.WithAttributes(
							attribute.String(attrMsgType, msgTypeTxt),
							attribute.Int(attrMsgLength, len(msg)),
						))
					// Call OnMessage callback
					wsengine.wsclient.OnMessage(
						onMessagectx,
						wsengine.conn,
						wsengine.readMutex,
						cancelSession,
						wsengine.engineStopFunc,
						sessionId,
						msgType,
						msg)
					// End OnMessage span
					onMessageSpan.SetStatus(codes.Ok, codes.Ok.String())
					onMessageSpan.End()
					// End run span and loop
					span.End()
					span.SetStatus(codes.Ok, codes.Ok.String())
				}
			}
		}
	}
}

// # Description
//
// Stop the engine. Method will call OnClose callback and close connection if needed or required.
// Finally, method will create a new goroutine which will execute onEngineShutdown.
//
// Method MUST be called once when engine stops. It is up to the engine developper to ensure this.
//
// Method must perform shutdown even if provided context is already canceled.
//
// # Inputs
//
//   - ctx: Context used for tracing purpose.
//   - closeMessage: Close message received from the server or generated by engine, if any.
//   - skipWebsocketClose: Skip connection close (because connection is already closed).
func (wsengine *WebsocketEngine) shutdownEngine(
	ctx context.Context,
	closeMessage *wsclient.CloseMessageDetails,
	skipWebsocketClose bool) {
	// Create shutdown span
	ctx, span := wsengine.tracer.Start(ctx, spanEngineShutdown,
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.Bool(attrHasCloseMessage, (closeMessage != nil)),
			attribute.Bool(attrSkipCloseConnection, skipWebsocketClose),
			attribute.Bool(attrAutoReconnect, wsengine.engineCfgOpts.AutoReconnect),
		))
	defer span.End()
	// Create onClose span
	onCloseCtx, onCloseSpan := wsengine.tracer.Start(ctx, spanEngineOnClose,
		trace.WithSpanKind(trace.SpanKindInternal))
	// Call onClose
	cmsg := wsengine.wsclient.OnClose(onCloseCtx, wsengine.conn, wsengine.readMutex, closeMessage)
	onCloseSpan.SetStatus(codes.Ok, codes.Ok.String())
	onCloseSpan.End()
	// Skip close if instructed to
	if !skipWebsocketClose {
		if cmsg == nil {
			// Set default close message if needed
			cmsg = &wsclient.CloseMessageDetails{
				CloseReason:  adapters.GoingAway,
				CloseMessage: "Going away",
			}
		}
		// Add an event to span with close message details
		span.AddEvent(eventConnectionClosed, trace.WithAttributes(
			attribute.String(attrCloseReason, cmsg.CloseMessage),
			attribute.Int(attrCloseCode, int(cmsg.CloseReason)),
		))
		// Close websocket connection
		err := wsengine.conn.Close(ctx, cmsg.CloseReason, cmsg.CloseMessage)
		if err != nil {
			// Record close error
			span.RecordError(err)
			// Start span for OnCloseError call
			onCloseErrorCtx, onCloseErrorSpan := wsengine.tracer.Start(ctx, spanEngineOnCloseError,
				trace.WithSpanKind(trace.SpanKindInternal))
			// Call OnWebsocketConnectionCloseError callback
			wsengine.wsclient.OnCloseError(onCloseErrorCtx, err)
			// Close span
			onCloseErrorSpan.SetStatus(codes.Ok, codes.Ok.String())
			onCloseErrorSpan.End()
		}
	}
	// Set shutdown span status to OK and move on
	span.SetStatus(codes.Ok, codes.Ok.String())
	// Finally, check whether engine has to be restarted or not
	if wsengine.engineCfgOpts.AutoReconnect {
		// Check on engine context to know if engine can restart
		select {
		case <-wsengine.engineCtx.Done():
			// Send signal on stopped channel -> the engine has finished stopping
			span.RecordError(wsengine.engineCtx.Err())
			span.AddEvent(eventEngineExit)
			wsengine.stoppedChannel <- true
		default:
			// Create a separate goroutine which will restart the engine. This goroutine will exit
			// and finish the shutdown phase.
			go wsengine.restartEngine(wsengine.engineCtx, wsengine.stoppedChannel, wsengine.engineStopFunc)
		}
	} else {
		// Send signal on stopped channel -> the engine has finished stopping
		span.AddEvent(eventEngineExit)
		wsengine.stoppedChannel <- true
	}
}

// # Description
//
// Continuously restart the websocket engine until it successfully restart. The method will block until either
// engine restarts or the engine is stopped by the user.
func (wsengine *WebsocketEngine) restartEngine(
	ctx context.Context,
	stoppedChannel chan bool,
	exit context.CancelFunc,
) {
	// Create restart context from engine context
	ctx, span := wsengine.tracer.Start(ctx, spanEngineRestart,
		trace.WithSpanKind(trace.SpanKindInternal))
	defer span.End()
	// Continuously try to restart until engine restarts or engine context is canceled
	retryCount := 0
	for {
		select {
		case <-ctx.Done():
			// Send signal on stopped channel as the engine will definitly stop
			stoppedChannel <- true
			// Exit
			span.RecordError(ctx.Err())
			span.AddEvent(eventEngineExit)
			span.SetStatus(codes.Ok, codes.Ok.String())
			return
		default:
			if retryCount > 0 {
				// Exponential retry delay
				delay := int(math.Ceil(math.Pow(
					float64(wsengine.engineCfgOpts.AutoReconnectRetryDelayBaseSeconds),
					math.Min(
						float64(retryCount),
						float64(wsengine.engineCfgOpts.AutoReconnectRetryDelayMaxExponent)))))
				time.Sleep(time.Duration(delay) * time.Second)
			}
			// If enabled, create a subcontext with timeout for start operation
			timeoutCtx := ctx
			cancel := func() {}
			if wsengine.engineCfgOpts.OnOpenTimeoutMs > 0 {
				timeoutCtx, cancel = context.WithTimeout(
					ctx,
					time.Duration(wsengine.engineCfgOpts.OnOpenTimeoutMs*int64(time.Millisecond)))
			}
			// Create internal channel to wait for the engine start completion signal
			startupChannel := make(chan error, 1)
			// Start a goroutine that will kick off the websocket engine.
			go wsengine.startEngine(timeoutCtx, true, startupChannel, exit)
			// Read from error channel or context done channel to know when the engine has finished
			// starting or if a timeout has occured or if engine context has been canceled.
			var err error
			select {
			case err = <-startupChannel:
				// Pass
			case <-timeoutCtx.Done():
				// A timeout has occured or provided parent context has been canceled.
				err = timeoutCtx.Err()
			}
			// Call cancel to trigger noop or prevent timeout from occuring
			cancel()
			// Engine has finished starting and sent back either a nil value (OK) or an error.
			if err != nil {
				// An error occured while engine was restarting - Record error
				span.RecordError(err)
				// Start span for OnRestartError
				onRestartErrorCtx, onRestartErrorSpan := wsengine.tracer.Start(
					timeoutCtx,
					spanEngineOnRestartError,
					trace.WithSpanKind(trace.SpanKindInternal),
					trace.WithAttributes(
						attribute.Int(attrRetryCount, retryCount),
					))
				// Call OnRestartError
				wsengine.wsclient.OnRestartError(
					onRestartErrorCtx,
					wsengine.engineStopFunc,
					err,
					retryCount)
				// Close span
				onRestartErrorSpan.SetStatus(codes.Ok, codes.Ok.String())
				onRestartErrorSpan.End()
				// Let loop
				retryCount = retryCount + 1
			} else {
				// Engine has started - Exit
				span.SetStatus(codes.Ok, codes.Ok.String())
				return
			}
		}
	}
}
