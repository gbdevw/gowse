// This package contains the implementation of a simple websocket server with the following features:
//   - subscribe/unsubcribe to a publication
//   - publish heartbeats from server to client who subscribed the publciation on a regular basis
//   - echo with optional user provided request ID. User can also request an error to be returned
//   - interrupt/close client connections
package demowsserver

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"nhooyr.io/websocket"
)

// Structure for the websocket server
type DemoWebsocketServer struct {
	// Underlying hhtp.Server
	httpServer *http.Server
	// Client sessions
	sessions map[string]*clientSession
	// Unix timestamp (seconds) when the server has started
	startUnixTimestamp int64
	// Total number of opened connection since server has started
	openedConnectionsCount int64
	// Indicates that server has started
	started bool
	// Buffered channel used to signal websocket server has finished stopping (Graceful shutdown)
	stoppedChannel chan error
	// Root context
	rootCtx context.Context
	// Context bound to websocket server lifetime
	serverCtx context.Context
	// Cancel fucntion used to stop server
	cancelServerCtx context.CancelFunc
	// Tracer used to instrument server code
	tracer trace.Tracer
	// Reference to instruments used to record server metrics
	instruments *demoWebsocketServerInstruments
	// Used to ensure server stop routine is performed once
	onceStop sync.Once
	// Internal mutex used to coordinate start/stop
	startMu *sync.Mutex
}

// Internal structure used to retain references to instruments that record DemoWebsocketServer metrics.
type demoWebsocketServerInstruments struct {
	// Gauge that monitors the number of active connections
	activeConnectionsGauge metric.Int64ObservableGauge
	// Gauge that retains the server start time as a unix timestamp (seconds)
	startUnixGauge metric.Int64ObservableGauge
	// Gauge that monitors server Started state flag
	startedGauge metric.Int64ObservableGauge
	// Counter that monitors the total number of opened connections to the server during its lifetime
	connectionsCounter metric.Int64ObservableCounter
}

// Internal structure that represents a client session
type clientSession struct {
	// Client session ID
	sessionId string
	// Client connection
	conn *websocket.Conn
	// When session has started
	startTimestamp time.Time
	// Context bound to client session lifetime
	clientCtx context.Context
	// Indicates that client has subscribed to heartbeat publication
	heartbeatSubscribed bool
	// Cancel function used to shutdown a client connection and all publications
	cancelClientSession context.CancelFunc
	// Cancel function used to stop a heartbeat publication - Defaults to noop
	cancelHeartbeatPublication context.CancelFunc
	// Buffered channel used to instruct to close a client connection with the provided close message
	closeChannel chan CloseMessage
	// Internal mutex used to coordinate when some state is updated
	mu *sync.Mutex
	// tracer used to instrument code
	tracer trace.Tracer
}

// Data of a websocket close message
type CloseMessage struct {
	// Close message reason code
	Code websocket.StatusCode
	// Close message reason
	Reason string
}

// Internal struct that will be used as receiver for errors when it is not possible to directly return the error
type errReceiver struct {
	// Received error
	receivedError error
}

// # Description
//
// Factory which creates a new, non-started DemoWebsocketServer.
//
// # Inputs
//
//   - ctx: Parent context to use as root context. All subcontextes will derive from this context and will be bound to this context lifecycle.
//   - httpServer: The underlying HTTP Server to use. The provided HTTP Server handler will be overriden with this server handler. If nil is provided, a default HTTP server listening on localhost:8080 will be used.
//   - tracerProvider: Tracer provider to use to get the tracer that will be used by the server. If nil, the global tracer provider will be used.
//   - meterProvider: Meter provider to use to get the meter that will be used by the server. If nil, the meter tracer provider will be used.
//
// # Returns
//
// A new, non-started DemoWebsocketServer or an error if any has occured.
func NewDemoWebsocketServer(ctx context.Context, httpServer *http.Server, tracerProvider trace.TracerProvider, meterProvider metric.MeterProvider) (*DemoWebsocketServer, error) {
	if httpServer == nil {
		// Check provided http.Server is not nil
		httpServer = &http.Server{Addr: "localhost:8080", BaseContext: func(l net.Listener) context.Context { return ctx }}
	}
	if tracerProvider == nil {
		// Use the global tracer provider if none is provided
		tracerProvider = otel.GetTracerProvider()
	}
	if meterProvider == nil {
		// Use the global meter provider if none is provided
		meterProvider = otel.GetMeterProvider()
	}
	// Get a meter provider
	meter := meterProvider.Meter(demowsserver_instrumentation_id)
	// Create cancelable server context
	srvCtx, srvCancel := context.WithCancel(ctx)
	// Build server with initial state
	srv := &DemoWebsocketServer{
		httpServer:             httpServer,
		sessions:               map[string]*clientSession{},
		startUnixTimestamp:     time.Now().Unix(),
		openedConnectionsCount: 0,
		started:                false,
		stoppedChannel:         make(chan error, 1),
		rootCtx:                ctx,
		serverCtx:              srvCtx,
		cancelServerCtx:        srvCancel,
		tracer:                 tracerProvider.Tracer(demowsserver_instrumentation_id),
		instruments:            &demoWebsocketServerInstruments{},
		onceStop:               sync.Once{},
		startMu:                &sync.Mutex{},
	}
	// Create and configure gauge that will watch the number of active connections
	activeConnectionsGauge, err := meter.Int64ObservableGauge(demowsserver_metric_active_connections_gauge, metric.WithInt64Callback(func(ctx context.Context, io metric.Int64Observer) error {
		count := 0
		for _, clientSession := range srv.sessions {
			select {
			case <-clientSession.clientCtx.Done():
				// Skip, dead connection
			default:
				count++
			}
		}
		io.Observe(int64(count))
		return nil
	}))
	if err != nil {
		return nil, err
	}
	// Create and configure gauge that will record the server start time as a unix timestamp (seconds)
	startUnixGauge, err := meter.Int64ObservableGauge(demowsserver_metric_start_unix_gauge, metric.WithUnit("seconds"), metric.WithInt64Callback(func(ctx context.Context, io metric.Int64Observer) error {
		io.Observe(srv.startUnixTimestamp)
		return nil
	}))
	if err != nil {
		return nil, err
	}
	// Create and configure gauge that will watch Started state flag (1 -> started | 0 -> not started)
	startedGauge, err := meter.Int64ObservableGauge(demowsserver_metric_started_gauge, metric.WithInt64Callback(func(ctx context.Context, io metric.Int64Observer) error {
		if srv.started {
			io.Observe(1)
		} else {
			io.Observe(0)
		}
		return nil
	}))
	if err != nil {
		return nil, err
	}
	// Create counter that records the total number of opened connections during server lifetime
	connectionsCounter, err := meter.Int64ObservableCounter(demowsserver_metric_connections_counter, metric.WithInt64Callback(func(ctx context.Context, io metric.Int64Observer) error {
		io.Observe(srv.openedConnectionsCount)
		return nil
	}))
	if err != nil {
		return nil, err
	}
	// Store references to insturments inside server struc. to prevent them from being GCed
	srv.instruments = &demoWebsocketServerInstruments{
		activeConnectionsGauge: activeConnectionsGauge,
		startUnixGauge:         startUnixGauge,
		startedGauge:           startedGauge,
		connectionsCounter:     connectionsCounter,
	}
	// Override http.Server handler
	srv.httpServer.Handler = srv
	// Retrun server
	return srv, nil
}

// # Description
//
// Start the websocket server that will accept incoming websocket connections.
func (srv *DemoWebsocketServer) Start() error {
	select {
	case <-srv.serverCtx.Done():
		// If server context is done -> do not start
		return fmt.Errorf("server context is done. A new server with a non-terminated context must be created")
	default:
		// Create span for sartup
		_, startSpan := srv.tracer.Start(srv.serverCtx, demowsserver_span_start, trace.WithSpanKind(trace.SpanKindServer), trace.WithAttributes(
			attribute.String(demowsserver_span_attr_start_host, srv.httpServer.Addr),
		))
		defer startSpan.End()
		// Lock start mutex
		srv.startMu.Lock()
		defer srv.startMu.Unlock()
		if srv.started {
			// Server already started -> error
			err := fmt.Errorf("server already started")
			log.Print(err)
			startSpan.RecordError(err)
			startSpan.SetStatus(codes.Error, codes.Error.String())
			return err
		} else {
			// Set started flag to true
			srv.started = true
			// Start a goroutine that cleanup dead client connections
			go srv.cleanupDeadClientSessions()
			// Start a goroutine that will run the http server
			go srv.httpServer.ListenAndServe()
			// Exit
			startSpan.SetStatus(codes.Ok, codes.Ok.String())
			return nil
		}
	}
}

// # Description
//
// Gracefully shutdown the websocket server. The method exits either when the server has finished stopping or when
// a timeout or another error has occured.
//
// # Returns
//
// Nil in case of success, an error otherwise.
func (srv *DemoWebsocketServer) Stop() error {
	select {
	case <-srv.rootCtx.Done():
		// If parent context is done -> do not do anything as all subcontextes will beterminated as well as all server resources and goroutines
		return fmt.Errorf("application context is done. A new server with a non-temrinated context must be created")
	default:
		// Begin Stop span
		ctx, stopSpan := srv.tracer.Start(srv.rootCtx, demowsserver_span_start, trace.WithSpanKind(trace.SpanKindServer))
		defer stopSpan.End()
		// Lock start mutex
		srv.startMu.Lock()
		defer srv.startMu.Unlock()
		// Check started flag
		if !srv.started {
			err := fmt.Errorf("server not started")
			stopSpan.RecordError(err)
			stopSpan.SetStatus(codes.Error, codes.Error.String())
			return err
		}
		// Use a receiver to capture stop error if any
		errReceiver := new(errReceiver)
		// Create context with timeout for terminate
		stopCtx, stopCancel := context.WithTimeout(ctx, srv_shutdown_timeout*time.Second)
		defer stopCancel()
		// Call once stop() method
		srv.onceStop.Do(func() { srv.terminate(stopCtx, errReceiver) })
		// Check error receiver
		if errReceiver.receivedError != nil {
			stopSpan.RecordError(errReceiver.receivedError)
			stopSpan.SetStatus(codes.Error, codes.Error.String())
			return errReceiver.receivedError
		}
		// Exit
		stopSpan.SetStatus(codes.Ok, codes.Ok.String())
		return nil
	}
}

// # Description
//
// Internal method which gracefully shutdowns the websocket server. The method exits either when the server has finished stopping or when
// an error has occured. In this later case, the method will return the error in the provided receiver.
//
// # Warning
//
// The internal method has been designed to be run once.
//
// # Inputs
//
//   - ctx: Context used for tracing purpose
//   - errReceiver: Receiver used to catch the error that might be returned from terminate
func (srv *DemoWebsocketServer) terminate(ctx context.Context, errRcv *errReceiver) {
	// Set Started flag to false
	srv.started = false
	// Create span for terminate
	stopCtx, stopSpan := srv.tracer.Start(ctx, demowsserver_span_terminate, trace.WithSpanKind(trace.SpanKindServer))
	defer stopSpan.End()
	// Cancel server context
	srv.cancelServerCtx()
	// Server shutdown
	err := srv.httpServer.Shutdown(stopCtx)
	if err != nil {
		// Record error (timeout or else)
		stopSpan.RecordError(err)
		stopSpan.SetStatus(codes.Error, codes.Error.String())
	} else {
		// Server has sutdown gracefully
		stopSpan.SetStatus(codes.Ok, codes.Ok.String())
	}
	errRcv.receivedError = err
}

// # Description
//
// Close all client connections if server is started. Otherwise, it is a noop. A close message that includes the provided code and reason will be sent.
//
// # Inputs
//
//   - closeMessage: Close message to send to clients before closing the connection on server side.
func (srv *DemoWebsocketServer) CloseClientConnections(closeMessage CloseMessage) {
	// Lock start mutex & check if server is started -> Noop
	srv.startMu.Lock()
	defer srv.startMu.Unlock()
	if !srv.started {
		return
	}
	// Close server connections
	select {
	case <-srv.serverCtx.Done():
		// Do nothing as all will be terminated soon
		return
	default:
		// Begin span
		_, span := srv.tracer.Start(srv.serverCtx, demowsserver_span_close_client_connections, trace.WithSpanKind(trace.SpanKindServer))
		defer span.End()
		defer span.SetStatus(codes.Ok, codes.Ok.String())
		// For each stored client session
		for _, session := range srv.sessions {
			select {
			case <-session.clientCtx.Done():
				// Session already closed -> skip
			default:
				// Write close message in CloseChannel so it can be picked up by the goroutine that manages client session
				session.closeChannel <- closeMessage
				// Cancel session context so the goroutine that manages client session can close the connection and exit
				session.cancelClientSession()
				// Add event
				span.AddEvent(demowsserver_event_closed_client_connections, trace.WithAttributes(
					attribute.String(demowsserver_event_attr_closed_client_connection_id, session.sessionId),
				))
			}
		}
	}
}

// # Description
//
// Server handler which accepts incoming websocket connections.
func (srv *DemoWebsocketServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Increase number of opened connections regardless the connection will finaly be accepted or not
	srv.openedConnectionsCount++
	// Create a cancellable context for the client session
	clientSessionCtx, cancelClientSession := context.WithCancel(srv.serverCtx)
	// Create a server span for Accept
	_, srvAcceptSpan := srv.tracer.Start(srv.serverCtx, demowsserver_span_accept, trace.WithSpanKind(trace.SpanKindServer))
	defer srvAcceptSpan.End()
	// Accept incoming client connection
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		// Log, record error in span, cancel client session context and exit
		log.Printf("an error occured while accepting client connection. Got %v", err)
		srvAcceptSpan.SetStatus(codes.Error, codes.Error.String())
		srvAcceptSpan.RecordError(err)
		cancelClientSession()
		return
	}
	// Create a random UUID that will identify client session
	uuid4, err := uuid.NewRandom()
	if err != nil {
		// Log, record error in span, cancel client session context and exit
		log.Printf("an error occured while generating a random UUID to identify client connection. Connection will be dropped. Got %v", err)
		srvAcceptSpan.SetStatus(codes.Error, codes.Error.String())
		srvAcceptSpan.RecordError(err)
		errClose := c.Close(websocket.StatusInternalError, "an error occured while generating a random UUID to identify client connection. Connection will be dropped.")
		srvAcceptSpan.RecordError(errClose)
		cancelClientSession()
		return
	}
	// Create client session
	clientSession := &clientSession{
		sessionId:           uuid4.String(),
		conn:                c,
		startTimestamp:      time.Now().UTC(),
		clientCtx:           clientSessionCtx,
		heartbeatSubscribed: false,
		cancelClientSession: cancelClientSession,
		cancelHeartbeatPublication: func() {
		},
		closeChannel: make(chan CloseMessage, 1),
		mu:           &sync.Mutex{},
		tracer:       srv.tracer,
	}
	// Put session in server session map
	srv.sessions[uuid4.String()] = clientSession
	// Start a goroutine that will manage client session
	go clientSession.run()
	// Exit - Success
	srvAcceptSpan.SetStatus(codes.Ok, codes.Ok.String())
}

// # Description
//
// Continuously scan client sessions an drop the ones with a closed client context.
func (srv *DemoWebsocketServer) cleanupDeadClientSessions() {
	// Count the number of loops performed after server shutdown has been instructed
	loopsAfterShutdown := 0
	for {
		// Create span for Cleanup
		_, cleanupSpan := srv.tracer.Start(srv.rootCtx, demowsserver_span_cleanup, trace.WithSpanKind(trace.SpanKindServer))
		select {
		case <-srv.serverCtx.Done():
			if loopsAfterShutdown >= srv_cleanup_max_loop_after_shutdown {
				// Signal all connections could not be closed by returning an error on stopped channel and exit
				err := fmt.Errorf("all client connections could not be closed. %d connections remaining", len(srv.sessions))
				srv.stoppedChannel <- err
				// Record err, exit event and end span
				cleanupSpan.RecordError(err)
				cleanupSpan.SetStatus(codes.Error, codes.Error.String())
				cleanupSpan.AddEvent(demowsserver_event_cleanup_exit, trace.WithAttributes(
					attribute.Int(demowsserver_event_cleanup_exit_loops_attr, loopsAfterShutdown),
				))
				cleanupSpan.End()
				return
			}
			if len(srv.sessions) == 0 {
				// Signal graceful shutdown is complete by sending nil on stopped channel and exit
				srv.stoppedChannel <- nil
				// Record exit event and end span
				cleanupSpan.SetStatus(codes.Ok, codes.Ok.String())
				cleanupSpan.AddEvent(demowsserver_event_cleanup_exit, trace.WithAttributes(
					attribute.Int(demowsserver_event_cleanup_exit_loops_attr, loopsAfterShutdown),
				))
				cleanupSpan.End()
				return
			}
			// Increase number of loops after shutdown and loop
			loopsAfterShutdown++
		default:
			// Count the number of removed sessions
			count := 0
			// Remove closed sessions
			for key, session := range srv.sessions {
				select {
				case <-session.clientCtx.Done():
					delete(srv.sessions, key)
					count++
				default:
					// Skip
				}
			}
			// Add event to span to record the number of removed sessions
			cleanupSpan.AddEvent(demowsserver_event_cleanup, trace.WithAttributes(
				attribute.Int(demowsserver_event_cleanup_count_attr, count),
			))
			// End span
			cleanupSpan.SetStatus(codes.Ok, codes.Ok.String())
			cleanupSpan.End()
		}
		// Sleep 15 secs before a new round
		time.Sleep(15 * time.Second)
	}
}

/*****************************************************************************/
/* CLIENT SESSION MANAGEMENT                                                 */
/*****************************************************************************/

// # Description
//
// Run & manages client session
func (session *clientSession) run() {
	for { // Run until session is cancelled or connection is closed
		// Create a new span for the iteration with a fresh context
		runCtx, runSpan := session.tracer.Start(session.clientCtx, demowsserver_span_client_session_run, trace.WithSpanKind(trace.SpanKindServer), trace.WithAttributes(
			attribute.String(demowsserver_span_client_session_run_id_attr, session.sessionId),
		))
		select {
		case <-session.clientCtx.Done():
			// Session has been canceled - Try to pick close message to send from close channel
			closeMsg := new(CloseMessage)
			select {
			case *closeMsg = <-session.closeChannel:
				// We got a close message to send
			default:
				// Otherwise, use a default close message
				closeMsg = &CloseMessage{
					Code:   websocket.StatusGoingAway,
					Reason: "server shutdown",
				}
			}
			// Send close message to client and exit
			err := session.conn.Close(closeMsg.Code, closeMsg.Reason)
			if err != nil {
				// Record error and set span status
				runSpan.RecordError(err)
				runSpan.SetStatus(codes.Error, codes.Error.String())
			} else {
				runSpan.SetStatus(codes.Ok, codes.Ok.String())
			}
			// Add exit event, close span and exit
			runSpan.AddEvent(demowsserver_event_client_session_exit)
			runSpan.End()
			return
		default:
			// Read message
			msgType, msg, err := session.conn.Read(session.clientCtx)
			select {
			case <-session.clientCtx.Done():
				// Session context has been cancelled. Let loop to close websocket connection and exit
				runSpan.SetStatus(codes.Ok, codes.Ok.String())
				runSpan.End()
			default:
				// Check error
				if err != nil {
					// Check if error is a close error
					if websocket.CloseStatus(err) == -1 {
						// Trace error
						runSpan.RecordError(err)
						runSpan.SetStatus(codes.Error, codes.Error.String())
						// Close client connection -> internal error
						err = session.conn.Close(websocket.StatusInternalError, "internal server error. Connection will be dropped")
						runSpan.RecordError(err)
					} else {
						// Set status as OK as error might have been caused by normal client exit (client closed the websocket conenction)
						runSpan.SetStatus(codes.Ok, codes.Ok.String())
						// Add exit event with close code and reason
						runSpan.AddEvent(demowsserver_event_client_session_exit, trace.WithAttributes(
							attribute.Int(demowsserver_event_client_session_exit_reason_code_attr, int(websocket.CloseStatus(err))),
						))
					}
					// Exit
					runSpan.End()
					session.cancelClientSession()
					return
				}
				// Process message
				session.handleMessage(runCtx, session.sessionId, msgType, msg)
				// Set span to OK and loop
				runSpan.SetStatus(codes.Ok, codes.Ok.String())
				runSpan.End()
			}
		}
	}
}

// # Description
//
// Function called to handle a single message from the client.
//
// # Inputs
//
//   - ctx: Parent context used for tracing purpose.
//   - sessionId: Client session ID
//   - msgType: Websocket message type (text|binary)
//   - message: Received message
func (session *clientSession) handleMessage(ctx context.Context, sessionId string, msgType websocket.MessageType, message []byte) {
	// Create span
	ctx, handleSpan := session.tracer.Start(ctx, demowsserver_span_client_session_run_handle, trace.WithSpanKind(trace.SpanKindServer), trace.WithAttributes(
		attribute.String(demowsserver_span_attr_client_session_id, session.sessionId),
	))
	defer handleSpan.End()
	defer handleSpan.SetStatus(codes.Ok, codes.Ok.String())
	// Unmarshal in base Message to be able to extract the message type
	base := new(Message)
	err := json.Unmarshal(message, base)
	if err != nil {
		// Trace error, write back and exit
		handleSpan.RecordError(err)
		log.Printf("could not not unmarshal message: %s", err.Error())
		session.WriteErrorMessage(ctx, fmt.Sprintf("could not find type field in message: %s", err.Error()), ERROR_UNKNOWN_MESSAGE_TYPE, "")
		return
	}
	// If echo request
	if base.MsgType == MSG_TYPE_ECHO_REQUEST {
		echoReq := new(EchoRequest)
		err := json.Unmarshal(message, echoReq)
		if err != nil {
			// Trace, log, write back and exit
			handleSpan.RecordError(err)
			log.Printf("could not unmarshal message to EchoRequest: %s", err.Error())
			session.WriteErrorMessage(ctx, fmt.Sprintf("malformatted EchoRequest: %s", err.Error()), ERROR_BAD_REQUEST, echoReq.ReqId)
			return
		}
		// Handle echo and exit
		session.handleEcho(ctx, echoReq)
		return
	}
	// If subscribe request
	if base.MsgType == MSG_TYPE_SUBSCRIBE_REQUEST {
		subscribeReq := new(SubscribeRequest)
		err := json.Unmarshal(message, subscribeReq)
		if err != nil {
			// Trace, log, write back and exit
			handleSpan.RecordError(err)
			log.Printf("could not unmarshal message to SubscribeRequest: %s", err.Error())
			session.WriteErrorMessage(ctx, fmt.Sprintf("malformatted SubscribeRequest: %s", err.Error()), ERROR_BAD_REQUEST, subscribeReq.ReqId)
			return
		}
		// Handle subscribe and exit
		session.handleSubscribe(ctx, subscribeReq)
		return
	}
	// If unsubscribe request
	if base.MsgType == MSG_TYPE_UNSUBSCRIBE_REQUEST {
		unsubscribeReq := new(UnsubscribeRequest)
		err := json.Unmarshal(message, unsubscribeReq)
		if err != nil {
			// Trace, log, write back and exit
			handleSpan.RecordError(err)
			log.Printf("could not unmarshal message to UnsubscribeRequest: %s", err.Error())
			session.WriteErrorMessage(ctx, fmt.Sprintf("malformatted UnsubscribeRequest: %s", err.Error()), ERROR_BAD_REQUEST, unsubscribeReq.ReqId)
			return
		}
		// Handle subscribe and exit
		session.handleUnsubscribe(ctx, unsubscribeReq)
		return
	}
	// Default -> Log, write back error message and exit
	handleSpan.RecordError(fmt.Errorf("unknown message type: %s", base.MsgType))
	log.Printf("unknown message type: %s", base.MsgType)
	session.WriteErrorMessage(ctx, fmt.Sprintf("unknown message type: %s", base.MsgType), ERROR_UNKNOWN_MESSAGE_TYPE, "")
}

/*****************************************************************************/
/* FEATURE: ECHO                                                             */
/*****************************************************************************/

// # Description
//
// Handle a echo request from a client.
//
// # Inputs
//
//   - ctx: Parent context used for tracing purpose.
//   - req: Client request.
func (session *clientSession) handleEcho(ctx context.Context, req *EchoRequest) {
	select {
	case <-ctx.Done():
		// Exit immediatly when client context is Done
		return
	default:
		// Start span
		ctx, echoSpan := session.tracer.Start(ctx, demowsserver_span_echo, trace.WithSpanKind(trace.SpanKindServer), trace.WithAttributes(
			attribute.Bool(demowsserver_span_attr_echo_return_error, req.Err),
			attribute.String(demowsserver_span_attr_echo_req_id, req.ReqId),
		))
		defer echoSpan.End()
		defer echoSpan.SetStatus(codes.Ok, codes.Ok.String())
		if req.Err {
			// Send echo response with error and exit
			session.WriteEchoResponseMessage(ctx, RESPONSE_STATUS_ERROR, req.ReqId, nil, &ErrorMessageData{
				Message: "client asked for it",
				Code:    ERROR_ECHO_ASKED_BY_CLIENT,
			})
		} else {
			// Send echo response and exit
			session.WriteEchoResponseMessage(ctx, RESPONSE_STATUS_OK, req.ReqId, &EchoResponseData{Echo: req.Echo}, nil)
		}
	}
}

/*****************************************************************************/
/* FEATURE: SUBSCRIBE                                                        */
/*****************************************************************************/

// # Description
//
// Handle a subscribe request from a client.
//
// # Inputs
//
//   - ctx: Parent context used for tracing purpose.
//   - req: Client request.
func (session *clientSession) handleSubscribe(ctx context.Context, req *SubscribeRequest) {
	select {
	case <-ctx.Done():
		// Exit immediatly when client context is Done
		return
	default:
		// Start span
		ctx, subscribeSpan := session.tracer.Start(ctx, demowsserver_span_subscribe, trace.WithSpanKind(trace.SpanKindServer), trace.WithAttributes(
			attribute.String(demowsserver_span_attr_subscribe_topic, req.Topic),
			attribute.String(demowsserver_span_attr_subscribe_req_id, req.ReqId),
		))
		defer subscribeSpan.End()
		defer subscribeSpan.SetStatus(codes.Ok, codes.Ok.String())
		// Topic: heartbeat
		if req.Topic == TOPIC_HEARTBEAT {
			// Lock mutex and check state
			session.mu.Lock()
			defer session.mu.Unlock()
			if !session.heartbeatSubscribed {
				// Create a cancellable context for publication
				publicationCtx, cancelPublication := context.WithCancel(session.clientCtx)
				session.cancelHeartbeatPublication = cancelPublication
				// Create a goroutine that will publish heartbeats
				go session.subscribeHeartbeat(publicationCtx)
				// Write response -> success
				session.WriteSubscribeResponseMessage(ctx, RESPONSE_STATUS_OK, req.ReqId, &SubscribeResponseData{Topic: req.Topic}, nil)
			} else {
				// Write unsubscribe response with error message -> already subscribed
				subscribeSpan.RecordError(fmt.Errorf("topic %s has been already subscribed", req.Topic))
				session.WriteSubscribeResponseMessage(ctx, RESPONSE_STATUS_ERROR, req.ReqId, nil, &ErrorMessageData{
					Message: fmt.Sprintf("topic %s has been already subscribed", req.Topic),
					Code:    ERROR_ALREADY_SUBSCRIBED,
				})
			}
			// Exit
			return
		}
		// Default: Write unsubscribe response with error message -> TOPIC_NOT_FOUND
		subscribeSpan.RecordError(fmt.Errorf("topic: %s - is not known by the server", req.Topic))
		session.WriteSubscribeResponseMessage(ctx, RESPONSE_STATUS_ERROR, req.ReqId, nil, &ErrorMessageData{
			Message: fmt.Sprintf("topic: %s - is not known by the server", req.Topic),
			Code:    ERROR_TOPIC_NOT_FOUND,
		})
	}
}

// # Description
//
//	Handle a Unsubscribe request.
//
// # Inputs
//
//   - ctx: Parent context used for tracing purpose.
//   - req: Client request.
func (session *clientSession) handleUnsubscribe(ctx context.Context, req *UnsubscribeRequest) {
	select {
	case <-ctx.Done():
		// Exit immediatly when client context is Done
		return
	default:
		// Start span
		ctx, unsubscribeSpan := session.tracer.Start(ctx, demowsserver_span_unsubscribe, trace.WithSpanKind(trace.SpanKindServer), trace.WithAttributes(
			attribute.String(demowsserver_span_attr_unsubscribe_topic, req.Topic),
			attribute.String(demowsserver_span_attr_unsubscribe_req_id, req.ReqId),
		))
		defer unsubscribeSpan.End()
		defer unsubscribeSpan.SetStatus(codes.Ok, codes.Ok.String())
		// Topic: heartbeat
		if req.Topic == TOPIC_HEARTBEAT {
			// Lock mutex and check state
			session.mu.Lock()
			defer session.mu.Unlock()
			if session.heartbeatSubscribed {
				// Cancel publication context to stop goroutine
				session.cancelHeartbeatPublication()
				// Write response -> success
				session.WriteUnsubscribeResponseMessage(ctx, RESPONSE_STATUS_OK, req.ReqId, &UnsubscribeResponseData{Topic: req.Topic}, nil)
			} else {
				// Write unsubscribe response with error message -> already subscribed
				unsubscribeSpan.RecordError(fmt.Errorf("topic %s has not been subscribed", req.Topic))
				session.WriteUnsubscribeResponseMessage(ctx, RESPONSE_STATUS_ERROR, req.ReqId, nil, &ErrorMessageData{
					Message: fmt.Sprintf("topic %s has not been subscribed", req.Topic),
					Code:    ERROR_NOT_SUBSCRIBED,
				})
			}
			// Exit
			return
		}
		// Default: Write unsubscribe response with error message -> TOPIC_NOT_FOUND
		unsubscribeSpan.RecordError(fmt.Errorf("topic: %s - is not known by the server", req.Topic))
		session.WriteUnsubscribeResponseMessage(ctx, RESPONSE_STATUS_ERROR, req.ReqId, nil, &ErrorMessageData{
			Message: fmt.Sprintf("topic: %s - is not known by the server", req.Topic),
			Code:    ERROR_TOPIC_NOT_FOUND,
		})
	}
}

/*****************************************************************************/
/* FEATURE: HEARTBEAT                                                        */
/*****************************************************************************/

// # Description
//
// # Start client heartbeat publication
//
// # Inputs
//
// - publicationCtx: Context bound to session lifecycle used for tracing purpose and monitored to know when to interrupt publication
func (session *clientSession) subscribeHeartbeat(publicationCtx context.Context) {
	// Lock mutex and update state
	session.mu.Lock()
	session.heartbeatSubscribed = true
	session.mu.Unlock()
	// Run until publication is canceled
	for {
		select {
		case <-publicationCtx.Done():
			return
		default:
			// Start span
			_, span := session.tracer.Start(publicationCtx, demowsserver_span_heartbeat, trace.WithSpanKind(trace.SpanKindServer), trace.WithAttributes(
				attribute.String(demowsserver_span_attr_heartbeat_client_id, session.sessionId),
			))
			// Create heartbeat message
			msg, err := json.Marshal(Heartbeat{
				Message:   Message{MsgType: MSG_TYPE_HEARTBEAT},
				Timestamp: time.Now().UTC().String(),
				Uptime:    strconv.FormatInt(int64(time.Now().UTC().Sub(session.startTimestamp).Seconds()), 10),
			})
			if err != nil {
				// Log, trace, unsubscribe & exit
				span.RecordError(err)
				span.SetStatus(codes.Error, codes.Error.String())
				log.Printf("client %s - stopping heartbeat publication - could not publish heartbeat: %s", session.sessionId, err.Error())
				session.cancelHeartbeatPublication()
				span.End()
				return
			}
			// Send message
			err = session.conn.Write(publicationCtx, websocket.MessageText, msg)
			if err != nil {
				// Log, trace, unsubscribe & exit
				span.RecordError(err)
				log.Printf("client %s - stopping heartbeat publication - could not publish heartbeat: %s", session.sessionId, err.Error())
				session.cancelHeartbeatPublication()
				span.SetStatus(codes.Ok, codes.Ok.String())
				span.End()
				return
			}
			// Close span
			span.SetStatus(codes.Ok, codes.Ok.String())
			span.End()
			// Wait 15 seconds before publishing again
			time.Sleep(srv_cleanup_frequency_seconds * time.Second)
		}
	}
}

/*****************************************************************************/
/* UTILITIES                                                                 */
/*****************************************************************************/

// # Description
//
// Helper method used to write an error message to client.
//
// # Inputs
//
//   - ctx: Parent context used for tracing purpose.
//   - message: Error message content
//   - code: Error message code
func (session *clientSession) WriteErrorMessage(ctx context.Context, message string, code string, reqId string) {
	select {
	case <-ctx.Done():
		// Exit, client session is terminated
		return
	default:
		// Start span
		ctx, writeErrMsgSpan := session.tracer.Start(ctx, demowsserver_span_write_error_message, trace.WithSpanKind(trace.SpanKindServer), trace.WithAttributes(
			attribute.String(demowsserver_span_attr_error_message_code, code),
			attribute.String(demowsserver_span_attr_error_message, message),
		))
		defer writeErrMsgSpan.End()
		// Marshal error response message
		msg, err := json.Marshal(ErrorMessage{
			Message: Message{
				MsgType: MSG_TYPE_ERROR,
			},
			ErrorMessageData: ErrorMessageData{
				Message: message,
				Code:    code,
			},
		})
		// Check marshal error
		if err != nil {
			// Trace, log and exit
			writeErrMsgSpan.RecordError(err)
			writeErrMsgSpan.SetStatus(codes.Error, codes.Error.String()) // Here, we are in an internal error
			log.Printf("client %s - could not marshal error message: %s", session.sessionId, err.Error())
			return
		}
		// Write message to client & exit
		err = session.conn.Write(ctx, websocket.MessageText, msg)
		if err != nil {
			// Trace, log
			writeErrMsgSpan.RecordError(err)
			log.Printf("client %s - could not echo back client: %s", session.sessionId, err.Error())
		}
		// Set span status and exit
		writeErrMsgSpan.SetStatus(codes.Ok, codes.Ok.String())
	}
}

// # Description
//
// Helper method to write an echo response message to client.
//
// # Inputs
//
//   - ctx: Parent context used for tracing purpose.
//   - status: Value for status field of echo response.
//   - reqId: User provided request ID used as value for reqId field of echo response.
//   - echoData: Echo response payload. Can be nil.
//   - errMsg: Error message to include in response. Can be nil.
func (session *clientSession) WriteEchoResponseMessage(ctx context.Context, status string, reqId string, echoData *EchoResponseData, errMsg *ErrorMessageData) {
	select {
	case <-ctx.Done():
		// Exit, client session is terminated
		return
	default:
		// Start span
		ctx, writeEchoMsgSpan := session.tracer.Start(ctx, demowsserver_span_write_error_message, trace.WithSpanKind(trace.SpanKindServer), trace.WithAttributes(
			attribute.String(demowsserver_span_attr_write_echo_response_status, status),
			attribute.String(demowsserver_span_attr_write_echo_response_req_id, reqId),
		))
		defer writeEchoMsgSpan.End()
		// Marshal echo response message
		msg, err := json.Marshal(EchoResponse{
			Response: Response{
				Message: Message{
					MsgType: MSG_TYPE_ECHO_RESPONSE,
				},
				ReqId:  reqId,
				Status: status,
				Err:    errMsg,
			},
			Data: echoData,
		})
		// Check marshal error
		if err != nil {
			// Trace, log and exit
			writeEchoMsgSpan.RecordError(err)
			writeEchoMsgSpan.SetStatus(codes.Error, codes.Error.String())
			log.Printf("client %s - could not marshal echo response message: %s", session.sessionId, err.Error())
			return
		}
		// Write message to client & exit
		err = session.conn.Write(ctx, websocket.MessageText, msg)
		if err != nil {
			// Log & trace
			writeEchoMsgSpan.RecordError(err)
			log.Printf("client %s - could not send response: %s", session.sessionId, err.Error())
		}
		// Exit
		writeEchoMsgSpan.SetStatus(codes.Ok, codes.Ok.String())
	}
}

// # Description
//
// Helper method to write a subscribe response message to client.
//
// # Inputs
//
//   - ctx: Parent context used for tracing purpose.
//   - status: Value for status field of echo response.
//   - reqId: User provided request ID used as value for reqId field of response.
//   - respData: Subscribe resposne payload.
//   - errMsg: Error message to include in response. Can be nil.
func (session *clientSession) WriteSubscribeResponseMessage(ctx context.Context, status string, reqId string, respData *SubscribeResponseData, errMsg *ErrorMessageData) {
	select {
	case <-ctx.Done():
		// Exit, client session is terminated
		return
	default:
		// Start span
		ctx, writeSubscribeMsgSpan := session.tracer.Start(ctx, demowsserver_span_write_subscribe_response, trace.WithSpanKind(trace.SpanKindServer), trace.WithAttributes(
			attribute.String(demowsserver_span_write_subscribe_response_status, status),
			attribute.String(demowsserver_span_write_subscribe_response_req_id, reqId),
		))
		defer writeSubscribeMsgSpan.End()
		// Marshal subscribe response message
		msg, err := json.Marshal(SubscribeResponse{
			Response: Response{
				Message: Message{
					MsgType: MSG_TYPE_SUBSCRIBE_RESPONSE,
				},
				ReqId:  reqId,
				Status: status,
				Err:    errMsg,
			},
			Data: respData,
		})
		// Check marshal error
		if err != nil {
			// Trace, log and exit
			writeSubscribeMsgSpan.RecordError(err)
			writeSubscribeMsgSpan.SetStatus(codes.Error, codes.Error.String())
			log.Printf("client %s - could not marshal subscribe response message: %s", session.sessionId, err.Error())
			return
		}
		// Write message to client & exit
		err = session.conn.Write(ctx, websocket.MessageText, msg)
		if err != nil {
			// Log & trace
			writeSubscribeMsgSpan.RecordError(err)
			log.Printf("client %s - could not send response: %s", session.sessionId, err.Error())
		}
		// Exit
		writeSubscribeMsgSpan.SetStatus(codes.Ok, codes.Ok.String())
	}
}

// # Description
//
// Helper method to write a unsubscribe response message to client.
//
// # Inputs
//
//   - ctx: Parent context used for tracing purpose.
//   - status: Value for status field of echo response.
//   - reqId: User provided request ID used as value for reqId field of response.
//   - respData: Unsubscribe resposne payload.
//   - errMsg: Error message to include in response. Can be nil.
func (session *clientSession) WriteUnsubscribeResponseMessage(ctx context.Context, status string, reqId string, respData *UnsubscribeResponseData, errMsg *ErrorMessageData) {
	select {
	case <-ctx.Done():
		// Exit, client session is terminated
		return
	default:
		// Start span
		ctx, writeUnsubscribeMsgSpan := session.tracer.Start(ctx, demowsserver_span_write_unsubscribe_response, trace.WithSpanKind(trace.SpanKindServer), trace.WithAttributes(
			attribute.String(demowsserver_span_write_unsubscribe_response_status, status),
			attribute.String(demowsserver_span_write_unsubscribe_response_req_id, reqId),
		))
		defer writeUnsubscribeMsgSpan.End()
		// Marshal unsubscribe response message
		msg, err := json.Marshal(UnsubscribeResponse{
			Response: Response{
				Message: Message{
					MsgType: MSG_TYPE_UNSUBSCRIBE_RESPONSE,
				},
				ReqId:  reqId,
				Status: status,
				Err:    errMsg,
			},
			Data: respData,
		})
		// Check marshal error
		if err != nil {
			// Log, trace and exit
			writeUnsubscribeMsgSpan.RecordError(err)
			writeUnsubscribeMsgSpan.SetStatus(codes.Error, codes.Error.String())
			log.Printf("client %s - could not marshal unsubscribe response message: %s", session.sessionId, err.Error())
			return
		}
		// Write message to client & exit
		err = session.conn.Write(ctx, websocket.MessageText, msg)
		if err != nil {
			// Log & trace
			writeUnsubscribeMsgSpan.RecordError(err)
			log.Printf("client %s - could not send response: %s", session.sessionId, err.Error())
		}
		// Exit
		writeUnsubscribeMsgSpan.SetStatus(codes.Ok, codes.Ok.String())
	}
}
