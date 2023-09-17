package demowsserver

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.opentelemetry.io/otel/trace"
	"nhooyr.io/websocket"
)

/*************************************************************************************************/
/* TEST SUITE                                                                                    */
/*************************************************************************************************/

// Test suite used to test DemoWebsocketServer methods like Start, Stop, ...
type DemoWebsocketServerMethodsTestSuite struct {
	suite.Suite
}

// Run DemoWebsocketServerMethodsTestSuite test suite
func TestDemoWebsocketServerTestSuite(t *testing.T) {
	suite.Run(t, new(DemoWebsocketServerMethodsTestSuite))
}

// Test suite used to test DemoWebsocketServer features like echo, heartbeat, ...
type DemoWebsocketServerFeaturesTestSuite struct {
	suite.Suite
	srv *DemoWebsocketServer
}

// Run DemoWebsocketServerFeaturesTestSuite test suite
func TestDemoWebsocketServerFeaturesTestSuite(t *testing.T) {
	suite.Run(t, new(DemoWebsocketServerFeaturesTestSuite))
}

// DemoWebsocketServerFeaturesTestSuite - Before all tests
func (suite *DemoWebsocketServerFeaturesTestSuite) SetupSuite() {
	// Create server
	srv, err := NewDemoWebsocketServer(context.Background(), &http.Server{Addr: "localhost:8081"}, nil, nil)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), srv)
	// Start server
	err = srv.Start()
	require.NoError(suite.T(), err)
	// Assign server to suite
	suite.srv = srv
}

// DemoWebsocketServerFeaturesTestSuite - Before all tests
func (suite *DemoWebsocketServerFeaturesTestSuite) TearDownSuite() {
	// Stop server
	suite.srv.Stop()
}

/*************************************************************************************************/
/* TEST SERVER                                                                                   */
/*************************************************************************************************/

// # Description
//
// Test server Start/Stop methods. Test will succeed if server can start and then immediatly stop without any error.
func (suite *DemoWebsocketServerMethodsTestSuite) TestServerStartAndStop() {
	// Create server
	srv, err := NewDemoWebsocketServer(context.Background(), nil, nil, nil)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), srv)
	// Start server
	err = srv.Start()
	require.NoError(suite.T(), err)
	// Connect client
	rootCtx := context.Background()
	conn, res, err := websocket.Dial(rootCtx, "ws://"+srv.httpServer.Addr, nil)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), res)
	// Stop server
	err = srv.Stop()
	require.NoError(suite.T(), err)
	// Read close on client connection and expect it to be closed
	_, _, err = conn.Read(rootCtx)
	require.Error(suite.T(), err)
	require.Equal(suite.T(), int(websocket.StatusGoingAway), int(websocket.CloseStatus(err)))
}

// # Description
//
// Test server Start/Stop methods concurrency. Test will succeed if different goroutines server can start and then stop without any error.
func (suite *DemoWebsocketServerMethodsTestSuite) TestServerStartAndStopConcurrency() {
	// Create server
	srv, err := NewDemoWebsocketServer(context.Background(), nil, nil, nil)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), srv)
	// Start server async
	startErrReceiver := struct {
		Err error
	}{
		Err: nil,
	}
	go func() { startErrReceiver.Err = srv.Start() }()
	// Stop server async
	stopErrReceiver := struct {
		Err error
	}{
		Err: nil,
	}
	go func() { stopErrReceiver.Err = srv.Stop() }()
	require.NoError(suite.T(), stopErrReceiver.Err)
	require.NoError(suite.T(), startErrReceiver.Err)
}

// # Description
//
// Test server Start method. Test will succeed if server starts and then returns an error on second Start method call.
func (suite *DemoWebsocketServerMethodsTestSuite) TestServerStartErrorAlreadyStarted() {
	// Create server
	srv, err := NewDemoWebsocketServer(context.Background(), nil, nil, nil)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), srv)
	// Start server
	err = srv.Start()
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
// Test server Start method. Test will succeed if server starts returns an error when server context is Done.
func (suite *DemoWebsocketServerMethodsTestSuite) TestServerStartErrorCtxDone() {
	// Create canceled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	// Create server with canceled context
	srv, err := NewDemoWebsocketServer(ctx, nil, nil, nil)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), srv)
	// Start server - Must error
	err = srv.Start()
	require.Error(suite.T(), err)
}

// # Description
//
// Test server Stop method. Test will succeed if server stop returns an error when server context is Done.
func (suite *DemoWebsocketServerMethodsTestSuite) TestServerStopErrorCtxDone() {
	// Create canceled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	// Create server with canceled context
	srv, err := NewDemoWebsocketServer(ctx, nil, nil, nil)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), srv)
	// Stop server - Must error
	err = srv.Stop()
	require.Error(suite.T(), err)
}

// # Description
//
// Test server Stop method. Test will succeed if server stop returns an error when method is called while server has not started.
func (suite *DemoWebsocketServerMethodsTestSuite) TestServerStopErrorSrvNotStarted() {
	// Create server
	srv, err := NewDemoWebsocketServer(context.Background(), nil, nil, nil)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), srv)
	// Stop server
	err = srv.Stop()
	require.Error(suite.T(), err)
}

// # Description
//
// Test CloseClientConnections. Two fake sessions will be created within the server session pool: One with a terminated context and another without a terminated context.
// Test will succeed if CloseClientConnections run without error, if the provided close message is present in the second session stop channel and if the second session
// context has been terminated.
func (suite *DemoWebsocketServerMethodsTestSuite) TestCloseClientConnections() {

	// Create a session with a terminated context
	ctx1, cancel1 := context.WithCancel(context.Background())
	cancel1()
	clientSession1 := &clientSession{
		sessionId:           "1",
		conn:                nil,
		startTimestamp:      time.Now(),
		clientCtx:           ctx1,
		heartbeatSubscribed: false,
		cancelClientSession: func() {
		},
		cancelHeartbeatPublication: func() {
		},
		closeChannel: make(chan CloseMessage),
		mu:           &sync.Mutex{},
		tracer:       trace.NewNoopTracerProvider().Tracer("NotUsed"),
	}
	// Create a session without a terminated context
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	clientSession2 := &clientSession{
		sessionId:           "2",
		conn:                nil,
		startTimestamp:      time.Now(),
		clientCtx:           ctx2,
		heartbeatSubscribed: false,
		cancelClientSession: cancel2,
		cancelHeartbeatPublication: func() {
		},
		closeChannel: make(chan CloseMessage),
		mu:           &sync.Mutex{},
		tracer:       trace.NewNoopTracerProvider().Tracer("NotUsed"),
	}
	// Expected close message
	closeMsg := CloseMessage{
		Code:   websocket.StatusGoingAway,
		Reason: "Connection shutdown",
	}
	// Create server
	srv, err := NewDemoWebsocketServer(context.Background(), nil, nil, nil)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), srv)
	// Start server
	err = srv.Start()
	require.NoError(suite.T(), err)
	// Put sessions
	srv.sessions["1"] = clientSession1
	srv.sessions["2"] = clientSession2
	// Start a goroutine that will read close channel
	msgRcv := struct {
		Msg CloseMessage
	}{
		Msg: CloseMessage{},
	}
	go func() {
		msg := <-clientSession2.closeChannel
		msgRcv.Msg = msg
	}()
	// Close client connections
	srv.CloseClientConnections(closeMsg)
	// Check session 1
	require.Empty(suite.T(), clientSession1.closeChannel)
	// Check session 2
	require.Error(suite.T(), ctx2.Err()) // Check canceled
	require.Empty(suite.T(), clientSession2.closeChannel)
	require.Equal(suite.T(), closeMsg, msgRcv.Msg)
}

// # Description
//
// Test CloseClientConnections on a non-started server Test will succeed if method completes without error.
func (suite *DemoWebsocketServerMethodsTestSuite) TestCloseClientConnectionsOnNotStartedSrv() {
	// Create server
	srv, err := NewDemoWebsocketServer(context.Background(), nil, nil, nil)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), srv)
	// Close client connections
	srv.CloseClientConnections(CloseMessage{})
}

// # Description
//
// Test CloseClientConnections with a server with a terminated context.
func (suite *DemoWebsocketServerMethodsTestSuite) TestCloseClientConnectionsOnTerminatedCtx() {
	// Create cancelable context
	ctx, cancel := context.WithCancel(context.Background())
	// Create server
	srv, err := NewDemoWebsocketServer(ctx, nil, nil, nil)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), srv)
	// Start server
	err = srv.Start()
	require.NoError(suite.T(), err)
	// Cancel context
	cancel()
	// Close client connections
	srv.CloseClientConnections(CloseMessage{})
}

/*************************************************************************************************/
/* TEST FEATURES                                                                                 */
/*************************************************************************************************/

// # Description
//
// Test DemoWebsocketServer client connection. Test will succeed if a websocket client can open a connection
// to the server, ping the server and close the connection.
func (suite *DemoWebsocketServerFeaturesTestSuite) TestClientConnection() {
	// Parent context
	rootCtx := context.Background()
	// Connect to websocket server, ping and close connection
	conn, res, err := websocket.Dial(rootCtx, "ws://"+suite.srv.httpServer.Addr, nil)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), res)
	// Required to handle pongs when we do not read messages on client side
	connCtx := conn.CloseRead(rootCtx)
	// Ping
	err = conn.Ping(rootCtx)
	require.NoError(suite.T(), err)
	// Close
	err = conn.Close(websocket.StatusNormalClosure, "Going away")
	require.NoError(suite.T(), err)
	// Wait connCtx to be Done
	<-connCtx.Done()
	require.Error(suite.T(), connCtx.Err())
}

// # Description
//
// Test DemoWebsocketServer client connection interrupt. Test will succeed if a websocket client can open a connection
// to the server and then read the server close read message sent when server shutdown all client connections.
func (suite *DemoWebsocketServerFeaturesTestSuite) TestClientConnectionInterruption() {
	// Parent context
	rootCtx := context.Background()
	// Connect to websocket server, ping and close connection
	conn, res, err := websocket.Dial(rootCtx, "ws://"+suite.srv.httpServer.Addr, nil)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), res)
	// Required to handle pongs when we do not read messages on client side
	connCtx := conn.CloseRead(rootCtx)
	err = conn.Ping(connCtx)
	require.NoError(suite.T(), err)
	// Shutdown client connections
	suite.srv.CloseClientConnections(CloseMessage{Code: websocket.StatusAbnormalClosure, Reason: "Server shutdown"})
	// Check client has caught the close message
	<-connCtx.Done()
	require.Error(suite.T(), connCtx.Err())
}

// # Description
//
// Test DemoWebsocketServer echo feature. Test will succeed if a websocket client can open a connection
// to the server, send and receive an echo.
func (suite *DemoWebsocketServerFeaturesTestSuite) TestFeatureEcho() {
	// Parent context
	rootCtx := context.Background()
	// Connect to websocket server
	conn, res, err := websocket.Dial(rootCtx, "ws://"+suite.srv.httpServer.Addr, nil)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), res)
	// Send echo request
	req := EchoRequest{
		Request: Request{
			Message: Message{
				MsgType: MSG_TYPE_ECHO_REQUEST,
			},
			ReqId: "1",
		},
		Echo: "Hello",
		Err:  false,
	}
	raw, err := json.Marshal(req)
	require.NoError(suite.T(), err)
	err = conn.Write(rootCtx, websocket.MessageText, raw)
	require.NoError(suite.T(), err)
	// Read server response
	msgType, msg, err := conn.Read(rootCtx)
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), websocket.MessageText, msgType)
	// Unmarshal response
	response := new(EchoResponse)
	err = json.Unmarshal(msg, response)
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), req.Echo, response.Data.Echo)
	// Close client connection
	err = conn.Close(websocket.StatusNormalClosure, "Going away")
	require.NoError(suite.T(), err)
}

// # Description
//
// Test DemoWebsocketServer echo feature. Test will succeed if a websocket client can open a connection
// to the server, send and receive an echo error.
func (suite *DemoWebsocketServerFeaturesTestSuite) TestFeatureEchoError() {
	// Parent context
	rootCtx := context.Background()
	// Connect to websocket server
	conn, res, err := websocket.Dial(rootCtx, "ws://"+suite.srv.httpServer.Addr, nil)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), res)
	// Send echo request
	req := EchoRequest{
		Request: Request{
			Message: Message{
				MsgType: MSG_TYPE_ECHO_REQUEST,
			},
			ReqId: "1",
		},
		Echo: "Hello",
		Err:  true,
	}
	raw, err := json.Marshal(req)
	require.NoError(suite.T(), err)
	err = conn.Write(rootCtx, websocket.MessageText, raw)
	require.NoError(suite.T(), err)
	// Read server response
	msgType, msg, err := conn.Read(rootCtx)
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), websocket.MessageText, msgType)
	// Unmarshal response
	response := new(EchoResponse)
	err = json.Unmarshal(msg, response)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), response.Err)
	// Close client connection
	err = conn.Close(websocket.StatusNormalClosure, "Going away")
	require.NoError(suite.T(), err)
}

// # Description
//
// Test DemoWebsocketServer heartbeat feature. Test will succeed if a websocket client can open a connection
// to the server, subscribe ehartbeat publication, receive two heartbeats and unsubscribe. Test will also ensure
// an error message is returned when heartbeat is not subscribed.
func (suite *DemoWebsocketServerFeaturesTestSuite) TestFeatureHeartbeat() {
	// Parent context
	rootCtx := context.Background()
	// Connect to websocket server
	conn, res, err := websocket.Dial(rootCtx, "ws://"+suite.srv.httpServer.Addr, nil)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), res)
	// Try to unsubscribr will not subscribed
	unsubscribeReq := UnsubscribeRequest{
		Request: Request{
			Message: Message{
				MsgType: MSG_TYPE_UNSUBSCRIBE_REQUEST,
			},
			ReqId: "2",
		},
		Topic: MSG_TYPE_HEARTBEAT,
	}
	unsubscribeReqBytes, err := json.Marshal(unsubscribeReq)
	require.NoError(suite.T(), err)
	// Try to unsubscribe
	err = conn.Write(rootCtx, websocket.MessageText, unsubscribeReqBytes)
	require.NoError(suite.T(), err)
	// Read unsubscribe response with error
	msgType, raw, err := conn.Read(rootCtx)
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), websocket.MessageText, msgType)
	unsubscribeResponse := new(UnsubscribeResponse)
	err = json.Unmarshal(raw, unsubscribeResponse)
	require.NoError(suite.T(), err)
	require.Nil(suite.T(), unsubscribeResponse.Data)
	require.NotNil(suite.T(), unsubscribeResponse.Err)
	require.Equal(suite.T(), MSG_TYPE_UNSUBSCRIBE_RESPONSE, unsubscribeResponse.MsgType)
	require.Equal(suite.T(), RESPONSE_STATUS_ERROR, unsubscribeResponse.Status)
	require.Equal(suite.T(), ERROR_NOT_SUBSCRIBED, unsubscribeResponse.Err.Code)
	// Subscribe heartbeat
	req := SubscribeRequest{
		Request: Request{
			Message: Message{
				MsgType: MSG_TYPE_SUBSCRIBE_REQUEST,
			},
			ReqId: "2",
		},
		Topic: MSG_TYPE_HEARTBEAT,
	}
	raw, err = json.Marshal(req)
	require.NoError(suite.T(), err)
	err = conn.Write(rootCtx, websocket.MessageText, raw)
	require.NoError(suite.T(), err)
	// Read server response
	msgType, msg, err := conn.Read(rootCtx)
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), websocket.MessageText, msgType)
	// Unmarshal response
	response := new(SubscribeResponse)
	err = json.Unmarshal(msg, response)
	require.NoError(suite.T(), err)
	require.Nil(suite.T(), response.Err)
	require.NotNil(suite.T(), response.Data)
	require.Equal(suite.T(), req.ReqId, response.ReqId)
	require.Equal(suite.T(), RESPONSE_STATUS_OK, response.Status)
	require.Equal(suite.T(), MSG_TYPE_SUBSCRIBE_RESPONSE, response.MsgType)
	require.Equal(suite.T(), req.Topic, response.Data.Topic)
	// Read some heartbeats
	heartbeatCount := 0
	for {
		// Read heartbeat response
		msgType, raw, err := conn.Read(rootCtx)
		require.NoError(suite.T(), err)
		require.Equal(suite.T(), websocket.MessageText, msgType)
		// Unmarshal message
		msg := new(Message)
		err = json.Unmarshal(raw, msg)
		require.NoError(suite.T(), err)
		// Handle message
		if msg.MsgType == MSG_TYPE_HEARTBEAT {
			heartbeatCount = heartbeatCount + 1
			if heartbeatCount == 2 {
				// Unsubscribe after 2 heartbeats
				err = conn.Write(rootCtx, websocket.MessageText, unsubscribeReqBytes)
				require.NoError(suite.T(), err)
			}
		}
		if msg.MsgType == MSG_TYPE_UNSUBSCRIBE_RESPONSE {
			break
		}
	}
	// Close client connection
	err = conn.Close(websocket.StatusNormalClosure, "Going away")
	require.NoError(suite.T(), err)
}

// # Description
//
// Test DemoWebsocketServer features error cases. Test will succeed if a websocket client can open a connection
// to the server and
//   - Get an error is it subscribe/unsubcribe to a non-existing topic
//   - Send a message with an unknown message type
//   - Send a message that is not JSON
//   - Send malformatted echo, subscribe, unsubscribe requests
func (suite *DemoWebsocketServerFeaturesTestSuite) TestFeatureErrors() {
	// Parent context
	rootCtx := context.Background()
	// Connect to websocket server
	conn, res, err := websocket.Dial(rootCtx, "ws://"+suite.srv.httpServer.Addr, nil)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), res)
	// Send request with unknown mesage type
	req := Request{
		Message: Message{
			MsgType: "NotExisting",
		},
		ReqId: "1",
	}
	raw, err := json.Marshal(req)
	require.NoError(suite.T(), err)
	err = conn.Write(rootCtx, websocket.MessageText, raw)
	require.NoError(suite.T(), err)
	// Read server response
	msgType, msg, err := conn.Read(rootCtx)
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), websocket.MessageText, msgType)
	// Unmarshal & checkresponse
	response := new(ErrorMessage)
	err = json.Unmarshal(msg, response)
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), "", response.ReqId)
	require.Equal(suite.T(), MSG_TYPE_ERROR, response.MsgType)
	require.Equal(suite.T(), ERROR_UNKNOWN_MESSAGE_TYPE, response.Code)
	// Send message with not known format
	err = conn.Write(rootCtx, websocket.MessageText, []byte("{}"))
	require.NoError(suite.T(), err)
	// Read server response
	msgType, msg, err = conn.Read(rootCtx)
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), websocket.MessageText, msgType)
	// Unmarshal & checkresponse
	response = new(ErrorMessage)
	err = json.Unmarshal(msg, response)
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), MSG_TYPE_ERROR, response.MsgType)
	require.Equal(suite.T(), ERROR_UNKNOWN_MESSAGE_TYPE, response.Code)
	// Send subscribe request for unknown topic
	unsubscribeReq := UnsubscribeRequest{
		Request: Request{
			Message: Message{
				MsgType: MSG_TYPE_UNSUBSCRIBE_REQUEST,
			},
			ReqId: "2",
		},
		Topic: "NotExisting",
	}
	unsubscribeReqBytes, err := json.Marshal(unsubscribeReq)
	require.NoError(suite.T(), err)
	// Try to unsubscribe not existing topic
	err = conn.Write(rootCtx, websocket.MessageText, unsubscribeReqBytes)
	require.NoError(suite.T(), err)
	// Read unsubscribe response with error
	msgType, raw, err = conn.Read(rootCtx)
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), websocket.MessageText, msgType)
	unsubscribeResponse := new(UnsubscribeResponse)
	err = json.Unmarshal(raw, unsubscribeResponse)
	require.NoError(suite.T(), err)
	require.Nil(suite.T(), unsubscribeResponse.Data)
	require.NotNil(suite.T(), unsubscribeResponse.Err)
	require.Equal(suite.T(), MSG_TYPE_UNSUBSCRIBE_RESPONSE, unsubscribeResponse.MsgType)
	require.Equal(suite.T(), RESPONSE_STATUS_ERROR, unsubscribeResponse.Status)
	require.Equal(suite.T(), ERROR_TOPIC_NOT_FOUND, unsubscribeResponse.Err.Code)
	// Try to subscribe not existing topic
	subReq := SubscribeRequest{
		Request: Request{
			Message: Message{
				MsgType: MSG_TYPE_SUBSCRIBE_REQUEST,
			},
			ReqId: "2",
		},
		Topic: "NOT EXISTING",
	}
	raw, err = json.Marshal(subReq)
	require.NoError(suite.T(), err)
	err = conn.Write(rootCtx, websocket.MessageText, raw)
	require.NoError(suite.T(), err)
	// Read server response
	msgType, msg, err = conn.Read(rootCtx)
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), websocket.MessageText, msgType)
	// Unmarshal response
	subResponse := new(SubscribeResponse)
	err = json.Unmarshal(msg, subResponse)
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), RESPONSE_STATUS_ERROR, subResponse.Status)
	// Send malformatted requests
	err = conn.Write(rootCtx, websocket.MessageText, []byte(`{"type" : "echo_request", "echo": 42}`))
	require.NoError(suite.T(), err)
	// Read server response
	msgType, msg, err = conn.Read(rootCtx)
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), websocket.MessageText, msgType)
	// Unmarshal & check
	malformattedResponse := new(ErrorMessage)
	err = json.Unmarshal(msg, malformattedResponse)
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), ERROR_BAD_REQUEST, malformattedResponse.Code)
	// Send malformatted requests
	err = conn.Write(rootCtx, websocket.MessageText, []byte(`{"type" : "unsubscribe_request", "topic": 42}`))
	require.NoError(suite.T(), err)
	// Read server response
	msgType, msg, err = conn.Read(rootCtx)
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), websocket.MessageText, msgType)
	// Unmarshal & check
	malformattedResponse = new(ErrorMessage)
	err = json.Unmarshal(msg, malformattedResponse)
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), ERROR_BAD_REQUEST, malformattedResponse.Code)
	// Send malformatted requests
	err = conn.Write(rootCtx, websocket.MessageText, []byte(`{"type" : "subscribe_request", "topic": 42}`))
	require.NoError(suite.T(), err)
	// Read server response
	msgType, msg, err = conn.Read(rootCtx)
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), websocket.MessageText, msgType)
	// Unmarshal & check
	malformattedResponse = new(ErrorMessage)
	err = json.Unmarshal(msg, malformattedResponse)
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), ERROR_BAD_REQUEST, malformattedResponse.Code)
	// Close client connection
	err = conn.Close(websocket.StatusNormalClosure, "Going away")
	require.NoError(suite.T(), err)
}
