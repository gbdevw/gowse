package gorilla

import (
	"context"
	"io"
	"log"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/gbdevw/gowsclient/echowsserver"
	"github.com/gbdevw/gowsclient/wscengine/wsadapters"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

/*************************************************************************************************/
/* TEST SUITE                                                                                    */
/*************************************************************************************************/

type GorillaWebsocketConnectionAdapterTestSuite struct {
	suite.Suite
}

// Run GorillaWebsocketConnectionAdapterTestSuite test suite
func TestGorillaWebsocketConnectionAdapterTestSuite(t *testing.T) {
	suite.Run(t, new(GorillaWebsocketConnectionAdapterTestSuite))
}

/*************************************************************************************************/
/* UNIT TESTS                                                                                    */
/*************************************************************************************************/

// Test compliance with WebsocketConnectionAdapterInterface
func (suite *GorillaWebsocketConnectionAdapterTestSuite) TestInterfaceCompliance() {
	var instance any = NewGorillaWebsocketConnectionAdapter(nil, nil)
	_, ok := instance.(wsadapters.WebsocketConnectionAdapterInterface)
	require.True(suite.T(), ok)
}

// Test Dial when there is already an active connection
func (suite *GorillaWebsocketConnectionAdapterTestSuite) TestDialWhenAlreadyConnected() {
	// Start a echo server
	host := "localhost:8083"
	u, err := url.Parse("ws://" + host)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), u)
	srv := echowsserver.NewEchoWebsocketServer(&http.Server{Addr: host}, log.New(io.Discard, "", log.Flags()))
	require.NotNil(suite.T(), srv)
	err = srv.Start()
	require.NoError(suite.T(), err)
	time.Sleep(1 * time.Second)
	// Create an adapter
	adapter := NewGorillaWebsocketConnectionAdapter(nil, nil)
	require.NotNil(suite.T(), adapter)
	// Create a context with a 15sec timeout to perform the test
	timeoutCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	// Connect to server
	resp, err := adapter.Dial(timeoutCtx, *u)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), resp)
	// Connect to server again
	resp, err = adapter.Dial(timeoutCtx, *u)
	require.Error(suite.T(), err)
	require.Nil(suite.T(), resp)
	// Close connection
	err = adapter.Close(timeoutCtx, wsadapters.NormalClosure, "bye")
	require.NoError(suite.T(), err)
	// Stop server
	srv.Stop()
}

// Test Dial when there is no server
func (suite *GorillaWebsocketConnectionAdapterTestSuite) TestDialWithoutPeer() {
	// Create an adapter
	adapter := NewGorillaWebsocketConnectionAdapter(nil, nil)
	require.NotNil(suite.T(), adapter)
	// Test dial - expect an error
	host := "localhost:8083"
	u, err := url.Parse("ws://" + host)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), u)
	resp, err := adapter.Dial(context.Background(), *u)
	require.Error(suite.T(), err)
	require.Nil(suite.T(), resp)
}

// Test Read and Write methods by performing sending and reading multiple echo messages
func (suite *GorillaWebsocketConnectionAdapterTestSuite) TestEcho() {
	// Start a echo server
	host := "localhost:8083"
	u, err := url.Parse("ws://" + host)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), u)
	srv := echowsserver.NewEchoWebsocketServer(&http.Server{Addr: host}, log.New(io.Discard, "", log.Flags()))
	require.NotNil(suite.T(), srv)
	err = srv.Start()
	require.NoError(suite.T(), err)
	time.Sleep(1 * time.Second)
	// Create an adapter
	adapter := NewGorillaWebsocketConnectionAdapter(nil, nil)
	require.NotNil(suite.T(), adapter)
	// Create a context with a 15sec timeout to perform the test
	timeoutCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	// Connect to server
	resp, err := adapter.Dial(timeoutCtx, *u)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), resp)
	// Multiple echoes
	echo := []byte("hello world")
	for i := 0; i < 4; i = i + 1 {
		// Write to echo server
		err := adapter.Write(timeoutCtx, wsadapters.Text, echo)
		require.NoError(suite.T(), err)
		// Read reply and verify
		msgType, msg, err := adapter.Read(timeoutCtx)
		require.NoError(suite.T(), err)
		require.Equal(suite.T(), wsadapters.Text, msgType)
		require.Equal(suite.T(), echo, msg)
	}
	// Close connection
	err = adapter.Close(timeoutCtx, wsadapters.NormalClosure, "bye")
	require.NoError(suite.T(), err)
	// Stop server
	srv.Stop()
}

// Test Ping: Test will succeed if adapter can connect to a websocket server, write
// a Ping message and read Pong from server
func (suite *GorillaWebsocketConnectionAdapterTestSuite) TestPing() {
	// Start a echo server
	host := "localhost:8083"
	u, err := url.Parse("ws://" + host)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), u)
	srv := echowsserver.NewEchoWebsocketServer(&http.Server{Addr: host}, log.New(io.Discard, "", log.Flags()))
	require.NotNil(suite.T(), srv)
	err = srv.Start()
	require.NoError(suite.T(), err)
	time.Sleep(1 * time.Second)
	// Create an adapter
	adapter := NewGorillaWebsocketConnectionAdapter(nil, nil)
	require.NotNil(suite.T(), adapter)
	// Create a context with a 15sec timeout to perform the test
	timeoutCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	// Connect to server
	resp, err := adapter.Dial(timeoutCtx, *u)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), resp)
	// Start a goroutine that will read incoming messages and process control frames - Pong included
	notification := make(chan error)
	go func() {
		// Read so Pong can be processed
		msgType, msg, err := adapter.Read(timeoutCtx)
		require.ErrorAs(suite.T(), err, new(wsadapters.WebsocketCloseError))
		require.Less(suite.T(), int(msgType), 0)
		require.Empty(suite.T(), msg)
		// Notify goroutine terminates
		notification <- err
	}()
	// Ping server and expect it to succeed
	err = adapter.Ping(timeoutCtx)
	require.NoError(suite.T(), err)
	// Close conenction so Read can be interrupted
	expectedCloseCode := wsadapters.GoingAway
	expectedCloseReason := "close client connection"
	err = adapter.Close(timeoutCtx, expectedCloseCode, expectedCloseReason)
	require.NoError(suite.T(), err)
	// Wait until notification is received or timeout occurs
	select {
	case err := <-notification:
		// Verify received error and pass
		require.ErrorAs(suite.T(), err, new(wsadapters.WebsocketCloseError))
	case <-timeoutCtx.Done():
		// Timeout has occured before we receive notification from Read goroutine - Fail
		suite.Fail(timeoutCtx.Err().Error())
	}
	// Stop server
	srv.Stop()
}

// Test Ping returns an error when connection is closed while Ping is waiting on its pong notification.
func (suite *GorillaWebsocketConnectionAdapterTestSuite) TestPendingPingWhenConnClose() {
	// Start a echo server
	host := "localhost:8083"
	u, err := url.Parse("ws://" + host)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), u)
	srv := echowsserver.NewEchoWebsocketServer(&http.Server{Addr: host}, log.New(io.Discard, "", log.Flags()))
	require.NotNil(suite.T(), srv)
	err = srv.Start()
	require.NoError(suite.T(), err)
	time.Sleep(1 * time.Second)
	// Create an adapter
	adapter := NewGorillaWebsocketConnectionAdapter(nil, nil)
	require.NotNil(suite.T(), adapter)
	// Create a context with a 15sec timeout to perform the test
	timeoutCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	// Connect to server
	resp, err := adapter.Dial(timeoutCtx, *u)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), resp)
	// Start a goroutine that will read incoming messages and process control frames - Pong included
	notification := make(chan error)
	// Start a goroutine that will remain stuck on Ping (no read = no pong notification)
	go func() {
		// Ping server and expect a close error
		err = adapter.Ping(timeoutCtx)
		require.ErrorAs(suite.T(), err, new(wsadapters.WebsocketCloseError))
		// Notify goroutine terminates
		notification <- err
	}()
	// Close connection so Ping can be interrupted
	time.Sleep(1 * time.Second)
	err = adapter.Close(timeoutCtx, wsadapters.GoingAway, "close client connection")
	require.NoError(suite.T(), err)
	// Wait until notification is received or timeout occurs
	select {
	case err := <-notification:
		// Verify received error and pass
		require.ErrorAs(suite.T(), err, new(wsadapters.WebsocketCloseError))
	case <-timeoutCtx.Done():
		// Timeout has occured before we receive notification from Read goroutine - Fail
		suite.Fail(timeoutCtx.Err().Error())
	}
	// Stop server
	srv.Stop()
}

// Test Ping timeout
func (suite *GorillaWebsocketConnectionAdapterTestSuite) TestPingTimeout() {
	// Start a echo server
	host := "localhost:8083"
	u, err := url.Parse("ws://" + host)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), u)
	srv := echowsserver.NewEchoWebsocketServer(&http.Server{Addr: host}, log.New(io.Discard, "", log.Flags()))
	require.NotNil(suite.T(), srv)
	err = srv.Start()
	require.NoError(suite.T(), err)
	time.Sleep(1 * time.Second)
	// Create an adapter
	adapter := NewGorillaWebsocketConnectionAdapter(nil, nil)
	require.NotNil(suite.T(), adapter)
	// Create a context with a 3sec timeout to perform the test
	timeoutCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	// Connect to server
	resp, err := adapter.Dial(timeoutCtx, *u)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), resp)
	// Ping server and verify interuption because of timeout
	err = adapter.Ping(timeoutCtx)
	require.ErrorIs(suite.T(), err, timeoutCtx.Err())
	// Stop server
	srv.Stop()
}

// Test adapter methods when called without an active connection set
func (suite *GorillaWebsocketConnectionAdapterTestSuite) TestMethodsWithoutActiveConnection() {
	// Create adapter
	adapter := NewGorillaWebsocketConnectionAdapter(nil, nil)
	require.NotNil(suite.T(), adapter)
	// Test close
	err := adapter.Close(context.Background(), wsadapters.GoingAway, "")
	require.Error(suite.T(), err)
	// Test Ping
	err = adapter.Ping(context.Background())
	require.Error(suite.T(), err)
	// Test Read
	msgType, msg, err := adapter.Read(context.Background())
	require.Error(suite.T(), err)
	require.Less(suite.T(), msgType, 0)
	require.Empty(suite.T(), msg)
	// Test Write
	err = adapter.Write(context.Background(), wsadapters.Text, []byte("hello"))
	require.Error(suite.T(), err)
	// Test GetUnderlyingWebsocketConnection
	require.Nil(suite.T(), adapter.GetUnderlyingWebsocketConnection())
}

// Test adapter methods when called with a canceled context.
func (suite *GorillaWebsocketConnectionAdapterTestSuite) TestMethodsWithCanceledContext() {
	// Create canceled context
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	// Create adapter
	adapter := NewGorillaWebsocketConnectionAdapter(nil, nil)
	require.NotNil(suite.T(), adapter)
	// Test Dial
	resp, err := adapter.Dial(canceledCtx, url.URL{})
	require.Error(suite.T(), err)
	require.Nil(suite.T(), resp)
	// Test Read
	msgType, msg, err := adapter.Read(canceledCtx)
	require.Error(suite.T(), err)
	require.Empty(suite.T(), msg)
	require.Less(suite.T(), int(msgType), 0)
	// Test Write
	err = adapter.Write(canceledCtx, -1, nil)
	require.Error(suite.T(), err)
	// Test Ping
	err = adapter.Ping(canceledCtx)
	require.Error(suite.T(), err)
}

// Test GetUnderlyingWebsocketConnection with an active connection set
func (suite *GorillaWebsocketConnectionAdapterTestSuite) TestGetUnderlyingWebsocketConnectionWithActiveConnection() {
	// Start a echo server
	host := "localhost:8083"
	u, err := url.Parse("ws://" + host)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), u)
	srv := echowsserver.NewEchoWebsocketServer(&http.Server{Addr: host}, log.New(io.Discard, "", log.Flags()))
	require.NotNil(suite.T(), srv)
	err = srv.Start()
	require.NoError(suite.T(), err)
	time.Sleep(1 * time.Second)
	// Create an adapter
	adapter := NewGorillaWebsocketConnectionAdapter(nil, nil)
	require.NotNil(suite.T(), adapter)
	// Create a context with a 5sec timeout to perform the test
	timeoutCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	// Connect to server
	resp, err := adapter.Dial(timeoutCtx, *u)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), resp)
	// Get underlying websocket connection and type assert
	conn := adapter.GetUnderlyingWebsocketConnection()
	_, ok := conn.(*websocket.Conn)
	require.True(suite.T(), ok)
	// Stop server
	srv.Stop()
}

// Test propagateToFirstActiveListener when there are no active listeners
func (suite *GorillaWebsocketConnectionAdapterTestSuite) TestPropagateToFirstActiveListenerWithoutActiveListener() {
	// Create chan used to receive notification channels
	listeners := make(chan chan error, 1)
	// Send a notification channel that will not be read (no active listener)
	listeners <- make(chan error)
	// Test propagateToFirstActiveListener and ensure false is returned
	require.False(suite.T(), propagateToFirstActiveListener(listeners, nil))
}

// Test propagateToAllActiveListener when there is an inactive listener
func (suite *GorillaWebsocketConnectionAdapterTestSuite) TestPropagateToAllActiveListenerWithoutActiveListener() {
	// Create chan used to receive notification channels
	listeners := make(chan chan error, 1)
	// Send a notification channel that will not be read (no active listener)
	listeners <- make(chan error)
	// Test propagateToAllActiveListener
	propagateToAllActiveListener(listeners, nil)
}
