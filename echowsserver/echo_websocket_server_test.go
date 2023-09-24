package echowsserver

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"nhooyr.io/websocket"
)

/*************************************************************************************************/
/* TEST SUITE                                                                                    */
/*************************************************************************************************/

// Test suite for EchoWebsocketServer
type EchoWebsocketServerMethodsTestSuite struct {
	suite.Suite
}

// Run EchoWebsocketServerMethodsTestSuite test suite
func TestEchoWebsocketServerMethodsTestSuite(t *testing.T) {
	suite.Run(t, new(EchoWebsocketServerMethodsTestSuite))
}

/*************************************************************************************************/
/* ECHOWEBSOCKETSERVER - TESTS                                                                   */
/*************************************************************************************************/

// # Description
//
// Test server Start/Stop methods.
//
// Test will succeed if
//   - Server starts without error
//   - A websocket client connect to the server & perform a ping/pong
//   - Server stops without error
//   - New websocket client ping fails because connection is closed.
func (suite *EchoWebsocketServerMethodsTestSuite) TestServerStartAndStop() {
	// Create server
	srv := NewEchoWebsocketServer(nil)
	require.NotNil(suite.T(), srv)
	// Start server
	err := srv.Start()
	require.NoError(suite.T(), err)
	// Connect client
	conn, res, err := websocket.Dial(context.Background(), "ws://localhost:8080", nil)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), res)
	// Automatically process incoming control frames & ping
	conn.CloseRead(context.Background())
	err = conn.Ping(context.Background())
	require.NoError(suite.T(), err)
	// Stop server
	err = srv.Stop()
	require.NoError(suite.T(), err)
	// Pause before testing connection again
	time.Sleep(2 * time.Second)
	// Read close on client connection and expect it to be closed
	err = conn.Ping(context.Background())
	require.Error(suite.T(), err)
}

// # Description
//
// Test server Start method. Test will succeed if server starts and then returns an error on second
// Start method call.
func (suite *EchoWebsocketServerMethodsTestSuite) TestServerStartErrorAlreadyStarted() {
	// Create server
	srv := NewEchoWebsocketServer(nil)
	require.NotNil(suite.T(), srv)
	// Start server
	err := srv.Start()
	require.NoError(suite.T(), err)
	// Start server - Must error
	err = srv.Start()
	require.Error(suite.T(), err)
	// Stop server
	err = srv.Stop()
	require.NoError(suite.T(), err)
}

// # Description
//
// Test server Stop method. Test will succeed if server stop returns an error when method is called
// while server has not started.
func (suite *EchoWebsocketServerMethodsTestSuite) TestServerStopErrorSrvNotStarted() {
	// Create server
	srv := NewEchoWebsocketServer(nil)
	require.NotNil(suite.T(), srv)
	// Stop server
	err := srv.Stop()
	require.Error(suite.T(), err)
}

// # Description
//
// Test EchoWebsocketServer echo feature. Test will succeed if a websocket client can open a connection
// to the server, and send and receive multiple echo messages.
func (suite *EchoWebsocketServerMethodsTestSuite) TestEchoFeature() {
	// Create server
	srv := NewEchoWebsocketServer(nil)
	require.NotNil(suite.T(), srv)
	// Start server
	err := srv.Start()
	require.NoError(suite.T(), err)
	// Connect to websocket server, ping and close connection
	conn, res, err := websocket.Dial(context.Background(), "ws://localhost:8080", nil)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), res)
	for i := 0; i < 4; i = i + 1 {
		// Write echo message
		expected := "hello world"
		err = conn.Write(context.Background(), websocket.MessageText, []byte(expected))
		require.NoError(suite.T(), err)
		// Read response with a 15 sec timeout on read
		timeoutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		msgType, msg, err := conn.Read(timeoutCtx)
		require.NoError(suite.T(), err)
		require.Equal(suite.T(), websocket.MessageText, msgType)
		require.NotEmpty(suite.T(), msg)
		require.Equal(suite.T(), expected, string(msg))
	}
	// Close from client side
	err = conn.Close(websocket.StatusNormalClosure, "Going away")
	require.NoError(suite.T(), err)
	// Stop server
	err = srv.Stop()
	require.NoError(suite.T(), err)
}
