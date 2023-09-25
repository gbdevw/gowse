// This package contains the implementation of a simple echo websocket
package echowsserver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// Alias type sued as key in context for the session ID
type contextKey string

const (
	sessionId contextKey = "sessionId"
)

// Structure for the websocket server
type EchoWebsocketServer struct {
	// Underlying http.Server
	httpServer *http.Server
	// Websocket upgrader
	upgrader websocket.Upgrader
	// Indicates that server has started
	started bool
	// Context bound to websocket server lifetime
	serverCtx context.Context
	// Cancel fucntion used to stop server
	cancelServerCtx context.CancelFunc
	// Internal mutex used to coordinate start/stop
	startMu *sync.Mutex
	// Logger
	logger *log.Logger
}

// # Description
//
// Factory which creates a new, non-started EchoWebsocketServer.
//
// # Inputs
//
//   - httpServer: The underlying HTTP Server to use. The provided HTTP Server handler will be
//     overriden with this server handler. If nil is provided, a default HTTP server listening
//     on localhost:8080 will be used.
//
//   - logger: Logger to use. If nil, default logger will be used
//
// # Returns
//
// A new, non-started EchoWebsocketServer or an error if any has occured.
func NewEchoWebsocketServer(httpServer *http.Server, logger *log.Logger) *EchoWebsocketServer {
	if httpServer == nil {
		// Check provided http.Server is not nil
		httpServer = &http.Server{Addr: "localhost:8080", BaseContext: func(l net.Listener) context.Context { return context.Background() }}
	}
	if logger == nil {
		// Use default logger
		logger = log.Default()
	}
	// Build server with initial state
	wssrv := &EchoWebsocketServer{
		httpServer: httpServer,
		upgrader:   websocket.Upgrader{},
		started:    false,
		startMu:    &sync.Mutex{},
		logger:     logger,
	}
	// Register server as handler of the underlying http server
	httpServer.Handler = wssrv
	// Return server
	return wssrv
}

// # Description
//
// Start the websocket server that will accept incoming websocket connections.
func (srv *EchoWebsocketServer) Start() error {

	// Lock start mutex
	srv.startMu.Lock()
	defer srv.startMu.Unlock()
	if srv.started {
		// Server is already started -> error
		return fmt.Errorf("server already started")
	}
	// Create cancelable server context
	srv.serverCtx, srv.cancelServerCtx = context.WithCancel(context.Background())
	// Start the server
	srv.started = true
	go srv.httpServer.ListenAndServe()
	return nil
}

// # Description
//
// # Stop the websocket server
//
// # Returns
//
// Nil in case of success, an error otherwise.
func (srv *EchoWebsocketServer) Stop() error {
	// Lock start mutex
	srv.startMu.Lock()
	defer srv.startMu.Unlock()
	// Check started flag
	if !srv.started {
		return fmt.Errorf("server not started")
	}
	// Cancel server context to shutdown all goroutines
	srv.cancelServerCtx()
	// Close server
	return srv.httpServer.Close()
}

// # Description
//
// Server handler which accepts incoming websocket connections.
func (srv *EchoWebsocketServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Accept incoming client connection
	srv.logger.Println("new client connection")
	c, err := srv.upgrader.Upgrade(w, r, nil)
	if err != nil {
		// Log, record error in span, cancel client session context and exit
		srv.logger.Println("an error occured while accepting client connection", err)
		return
	}
	// Start goroutines which will handle new client
	go srv.closeWatchdog(srv.serverCtx, c)
	go srv.runClientSession(context.WithValue(srv.serverCtx, sessionId, uuid.New()), c)
}

// Manages the client session and handle echo feature until the conneciton is closed.
func (srv *EchoWebsocketServer) runClientSession(ctx context.Context, conn *websocket.Conn) {
	for {
		// Read message
		srv.logger.Printf("%s - waiting for messages"+"\n", ctx.Value(sessionId))
		mt, message, err := conn.ReadMessage()
		if err != nil {
			// Check if close error
			if ce, ok := err.(*websocket.CloseError); ok {
				// Connection is closed
				srv.logger.Printf("%s - connection closed: %s"+"\n", ctx.Value(sessionId), ce.Error())
				return
			}
			if errors.Is(err, io.EOF) ||
				strings.Contains(strings.ToLower(err.Error()), "use of closed network connection") {
				// Connection is closed
				srv.logger.Printf("%s - connection closed: %s"+"\n", ctx.Value(sessionId), err.Error())
				return
			}
			// Other errors
			srv.logger.Printf("%s - read error: %s"+"\n", ctx.Value(sessionId), err.Error())
			return
		}
		// Log received message
		srv.logger.Printf("%s - type: %d - read: %s"+"\n", ctx.Value(sessionId), mt, string(message))
		// Echo
		err = conn.WriteMessage(mt, message)
		if err != nil {
			srv.logger.Printf("%s - write error: %s"+"\n", ctx.Value(sessionId), err.Error())
			return
		}
	}
}

// This function waits for a cancelation signal on provided context Done channel
// and close the provided websocket connection
func (srv *EchoWebsocketServer) closeWatchdog(ctx context.Context, conn *websocket.Conn) {
	// Wait for context to be canceled
	<-ctx.Done()
	// Close connection
	conn.Close()
}
