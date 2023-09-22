package wsadapternhooyr

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/gbdevw/gowsclient/demowsserver"
	wsconnadapter "github.com/gbdevw/gowsclient/wscengine/wsadapters"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"nhooyr.io/websocket"
)

/*************************************************************************************************/
/* TEST SUITE                                                                                    */
/*************************************************************************************************/

type NhooyrWebsocketConnectionAdapterTestSuite struct {
	suite.Suite
	// Websocket server address
	srvUrl *url.URL
	// Websocket test server
	srv *demowsserver.DemoWebsocketServer
}

// Run NhooyrWebsocketConnectionAdapterTestSuite test suite
func TestNhooyrWebsocketConnectionAdapterTestSuite(t *testing.T) {
	suite.Run(t, new(NhooyrWebsocketConnectionAdapterTestSuite))
}

// NhooyrWebsocketConnectionAdapterTestSuite - Before all tests
func (suite *NhooyrWebsocketConnectionAdapterTestSuite) SetupSuite() {
	// Create server
	host := "localhost:8081"
	srv, err := demowsserver.NewDemoWebsocketServer(context.Background(), &http.Server{Addr: host}, nil, nil)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), srv)
	// Start server
	err = srv.Start()
	require.NoError(suite.T(), err)
	// Assign server to suite
	suite.srv = srv
	u, err := url.Parse("ws://" + host)
	require.NoError(suite.T(), err)
	suite.srvUrl = u
}

// NhooyrWebsocketConnectionAdapterTestSuite - Before all tests
func (suite *NhooyrWebsocketConnectionAdapterTestSuite) TearDownSuite() {
	// Stop server
	suite.srv.Stop()
}

/*************************************************************************************************/
/* TESTS                                                                                         */
/*************************************************************************************************/

// # Description
//
// Check adapter fully implements interface.
func (suite *NhooyrWebsocketConnectionAdapterTestSuite) TestInterfaceCompliance() {
	var impl interface{} = NewNhooyrWebsocketConnectionAdapter(nil, nil)
	_, ok := impl.(wsconnadapter.WebsocketConnectionAdapterInterface)
	require.True(suite.T(), ok)
}

// # Description
//
// Test the following adapter methods:
//   - Dial: normal usage + subsequent calls when connections is already up
//   - Ping: normal usage
//   - Close: normal usage + subsequent calls when connections is down
//   - GetUnderlyingWebsocketConnection: normal usage
//
// Test will succeed if:
//   - GetUnderlyingWebsocketConnection returns a nil websocket.Conn
//   - First Dial against test server succeed
//   - GetUnderlyingWebsocketConnection returns a non-nil websocket.Conn
//   - Received underlying connection can be used to send a ping
//   - Adapter can be used to send a ping
//   - Second Dial call succeed and cause first connection to be closed
//   - GetUnderlyingWebsocketConnection returns a non-nil websocket.Conn
//   - Received underlying connection can be used to send a ping
//   - Call to Close succeed and cause second connection to be closed
//   - Second call to Close fail
//   - GetUnderlyingWebsocketConnection returns a nil websocket.Conn
func (suite *NhooyrWebsocketConnectionAdapterTestSuite) TestControlMethods() {

	// Create adapter
	adapter := NewNhooyrWebsocketConnectionAdapter(nil, nil)
	// Get underlying websocket connection and check is nil
	uconnif := adapter.GetUnderlyingWebsocketConnection()
	require.Nil(suite.T(), uconnif)
	// Connect to the server
	_, err := adapter.Dial(context.Background(), suite.srvUrl)
	require.NoError(suite.T(), err)
	// Get underlying websocket connection & type assert
	uconnif = adapter.GetUnderlyingWebsocketConnection()
	require.NotNil(suite.T(), uconnif)
	uconn, ok := uconnif.(*websocket.Conn)
	require.True(suite.T(), ok)
	// Use underlying connection to Ping server
	uconn.CloseRead(context.Background()) // As no concurrent goroutine is here to read
	err = uconn.Ping(context.Background())
	require.NoError(suite.T(), err)
	// Use adapter to ping
	err = adapter.Ping(context.Background())
	require.NoError(suite.T(), err)
	// Call Dial again
	_, err = adapter.Dial(context.Background(), suite.srvUrl)
	require.NoError(suite.T(), err)
	// Check connection is closed
	err = uconn.Ping(context.Background())
	require.Error(suite.T(), err)
	// Get underlying websocket connection & type assert
	uconnif = adapter.GetUnderlyingWebsocketConnection()
	require.NotNil(suite.T(), uconnif)
	uconn, ok = uconnif.(*websocket.Conn)
	require.True(suite.T(), ok)
	// Use underlying connection to Ping server
	uconn.CloseRead(context.Background()) // As no concurrent goroutine is here to read
	err = uconn.Ping(context.Background())
	require.NoError(suite.T(), err)
	// Call Close
	err = adapter.Close(context.Background(), wsconnadapter.GoingAway, "")
	require.NoError(suite.T(), err)
	// Check connection is closed
	err = uconn.Ping(context.Background())
	require.Error(suite.T(), err)
	// Call Close a second time
	err = adapter.Close(context.Background(), wsconnadapter.GoingAway, "")
	require.Error(suite.T(), err)
	// Get underlying websocket connection and check is nil
	uconnif = adapter.GetUnderlyingWebsocketConnection()
	require.Nil(suite.T(), uconnif)
}

// # Description
//
// Test control methods error paths when no connection is set.
//
// Test will succeed if, given an adapter without active connection:
//   - Close return an error when called without active connection set.
//   - Ping return an error when called without active connection set.
//   - Read return an error when called without active connection set.
//   - Write return an error when called without active connection set.
func (suite *NhooyrWebsocketConnectionAdapterTestSuite) TestMethodsErrorPathsWhenNoConnectionIsSet() {
	// Create adapter
	adapter := NewNhooyrWebsocketConnectionAdapter(nil, nil)
	// Call Close
	err := adapter.Close(context.Background(), wsconnadapter.GoingAway, "")
	require.Error(suite.T(), err)
	// Call Ping
	err = adapter.Ping(context.Background())
	require.Error(suite.T(), err)
	// Call Read
	msgType, raw, err := adapter.Read(context.Background())
	require.Error(suite.T(), err)
	require.Equal(suite.T(), -1, int(msgType))
	require.Nil(suite.T(), raw)
	// Call Write
	err = adapter.Write(context.Background(), wsconnadapter.Text, []byte("Hello"))
	require.Error(suite.T(), err)
}

// # Description
//
// Test control methods error paths when called with a cancelled context.
//
// Test will succeed if, given an adapter without active connection:
//   - Close return an error when called  with a cancelled context.
//   - Ping return an error when called  with a cancelled context.
//   - Read return an error when called  with a cancelled context.
//   - Write return an error when called  with a cancelled context.
func (suite *NhooyrWebsocketConnectionAdapterTestSuite) TestMethodsErrorPathsWhenCtxIsCancelled() {
	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	// Create adapter
	adapter := NewNhooyrWebsocketConnectionAdapter(nil, nil)
	// Call Ping
	err := adapter.Ping(ctx)
	require.Error(suite.T(), err)
	require.EqualError(suite.T(), err, ctx.Err().Error())
	// Call Read
	msgType, raw, err := adapter.Read(ctx)
	require.Error(suite.T(), err)
	require.EqualError(suite.T(), err, ctx.Err().Error())
	require.Equal(suite.T(), -1, int(msgType))
	require.Nil(suite.T(), raw)
	// Call Write
	err = adapter.Write(ctx, wsconnadapter.Text, []byte("Hello"))
	require.Error(suite.T(), err)
	require.EqualError(suite.T(), err, ctx.Err().Error())
}

// # Description
//
// Test Dial method error path when trying to cnnect to a non-existing server.
func (suite *NhooyrWebsocketConnectionAdapterTestSuite) TestDialFailure() {
	// Create adapter
	adapter := NewNhooyrWebsocketConnectionAdapter(nil, nil)
	// Dial non-existing server
	u, err := url.Parse("ws://localhost:42/none")
	require.NoError(suite.T(), err)
	res, err := adapter.Dial(context.Background(), u)
	require.Error(suite.T(), err)
	require.Nil(suite.T(), res)
}

// # Description
//
// Test methods error paths when server server has closed connection and adapter still thinks
// connection is up.
//
// Test will succeed if:
//   - Adapter open a connection to the server and server close it with an close message.
//   - Adapter Read returns a close error.
//   - Adapter Ping returns an error.
//   - Adapter Write returns an error.
func (suite *NhooyrWebsocketConnectionAdapterTestSuite) TestMethodsErrorWhenSrvHasClosedConnection() {
	// Create adapter
	adapter := NewNhooyrWebsocketConnectionAdapter(nil, nil)
	// Connect to the server
	_, err := adapter.Dial(context.Background(), suite.srvUrl)
	require.NoError(suite.T(), err)
	// Close the connection from server side
	srvCloseMsg := demowsserver.CloseMessage{
		Code:   websocket.StatusGoingAway,
		Reason: "server shutdown",
	}
	suite.srv.CloseClientConnections(srvCloseMsg)
	// Read from connection
	_, _, err = adapter.Read(context.Background())
	errRcvr := new(wsconnadapter.WebsocketCloseError)
	require.ErrorAs(suite.T(), err, errRcvr)
	require.True(suite.T(), errRcvr.Code == wsconnadapter.GoingAway || errRcvr.Code == wsconnadapter.AbnormalClosure)
	require.NotEmpty(suite.T(), errRcvr.Reason)
	// Ping
	err = adapter.Ping(context.Background())
	require.Error(suite.T(), err)
	// Write
	err = adapter.Write(context.Background(), wsconnadapter.Text, []byte("Hello"))
	require.Error(suite.T(), err)
}

// # Description
//
// Test Read and Write methods against test server echo feature.
//
// Test will succeed if:
//   - Adapter can open a connection to the server
//   - Adapter write can send a EchoRequest to test server
//   - Adapter can read the EchoResponse from the server
//   - Adapter can close the connection to the server
func (suite *NhooyrWebsocketConnectionAdapterTestSuite) TestReadAndWrite() {
	// Create adapter
	adapter := NewNhooyrWebsocketConnectionAdapter(nil)
	// Connect to the server
	_, err := adapter.Dial(context.Background(), suite.srvUrl)
	require.NoError(suite.T(), err)
	// Format & marshal request
	echoReq := demowsserver.EchoRequest{
		Request: demowsserver.Request{
			Message: demowsserver.Message{
				MsgType: demowsserver.MSG_TYPE_ECHO_REQUEST,
			},
			ReqId: "42",
		},
		Echo: "Hello",
		Err:  false,
	}
	raw, err := json.Marshal(echoReq)
	require.NoError(suite.T(), err)
	// Write request to server
	err = adapter.Write(context.Background(), wsconnadapter.Text, raw)
	require.NoError(suite.T(), err)
	// Read response
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	msgType, raw, err := adapter.Read(ctx)
	cancel()
	require.NoError(suite.T(), err)
	// Unmarshal response
	echoResp := new(demowsserver.EchoResponse)
	err = json.Unmarshal(raw, echoResp)
	require.NoError(suite.T(), err)
	// Check response
	require.Equal(suite.T(), wsconnadapter.Text, msgType)
	require.Equal(suite.T(), echoReq.ReqId, echoResp.ReqId)
	require.Equal(suite.T(), demowsserver.MSG_TYPE_ECHO_RESPONSE, echoResp.MsgType)
	require.Equal(suite.T(), demowsserver.RESPONSE_STATUS_OK, echoResp.Status)
	require.NotNil(suite.T(), echoResp.Data.Echo)
	require.Nil(suite.T(), echoResp.Err)
	require.Equal(suite.T(), echoReq.Echo, echoResp.Data.Echo)
	// Close connection
	err = adapter.Close(context.Background(), wsconnadapter.GoingAway, "")
	require.NoError(suite.T(), err)
}

// Test status code helper functions.
func (suite *NhooyrWebsocketConnectionAdapterTestSuite) TestStatusCodeConverters() {
	// Test converter from nhooyr status codes
	require.Equal(suite.T(), wsconnadapter.AbnormalClosure, convertFromNhooyrStatusCodes(-1))
	require.Equal(suite.T(), wsconnadapter.AbnormalClosure, convertFromNhooyrStatusCodes(websocket.StatusAbnormalClosure))
	require.Equal(suite.T(), wsconnadapter.NormalClosure, convertFromNhooyrStatusCodes(websocket.StatusNormalClosure))
	require.Equal(suite.T(), wsconnadapter.GoingAway, convertFromNhooyrStatusCodes(websocket.StatusGoingAway))
	require.Equal(suite.T(), wsconnadapter.ProtocolError, convertFromNhooyrStatusCodes(websocket.StatusProtocolError))
	require.Equal(suite.T(), wsconnadapter.UnsupportedData, convertFromNhooyrStatusCodes(websocket.StatusUnsupportedData))
	require.Equal(suite.T(), wsconnadapter.NoStatusReceived, convertFromNhooyrStatusCodes(websocket.StatusNoStatusRcvd))
	require.Equal(suite.T(), wsconnadapter.InvalidFramePayloadData, convertFromNhooyrStatusCodes(websocket.StatusInvalidFramePayloadData))
	require.Equal(suite.T(), wsconnadapter.PolicyViolation, convertFromNhooyrStatusCodes(websocket.StatusPolicyViolation))
	require.Equal(suite.T(), wsconnadapter.MessageTooBig, convertFromNhooyrStatusCodes(websocket.StatusMessageTooBig))
	require.Equal(suite.T(), wsconnadapter.MandatoryExtension, convertFromNhooyrStatusCodes(websocket.StatusMandatoryExtension))
	require.Equal(suite.T(), wsconnadapter.InternalError, convertFromNhooyrStatusCodes(websocket.StatusInternalError))
	require.Equal(suite.T(), wsconnadapter.TLSHandshake, convertFromNhooyrStatusCodes(websocket.StatusTLSHandshake))
	// Test converter to nhooyr status codes
	require.Equal(suite.T(), websocket.StatusAbnormalClosure, convertToNhooyrStatusCodes(-1))
	require.Equal(suite.T(), websocket.StatusAbnormalClosure, convertToNhooyrStatusCodes(wsconnadapter.AbnormalClosure))
	require.Equal(suite.T(), websocket.StatusNormalClosure, convertToNhooyrStatusCodes(wsconnadapter.NormalClosure))
	require.Equal(suite.T(), websocket.StatusGoingAway, convertToNhooyrStatusCodes(wsconnadapter.GoingAway))
	require.Equal(suite.T(), websocket.StatusProtocolError, convertToNhooyrStatusCodes(wsconnadapter.ProtocolError))
	require.Equal(suite.T(), websocket.StatusUnsupportedData, convertToNhooyrStatusCodes(wsconnadapter.UnsupportedData))
	require.Equal(suite.T(), websocket.StatusNoStatusRcvd, convertToNhooyrStatusCodes(wsconnadapter.NoStatusReceived))
	require.Equal(suite.T(), websocket.StatusInvalidFramePayloadData, convertToNhooyrStatusCodes(wsconnadapter.InvalidFramePayloadData))
	require.Equal(suite.T(), websocket.StatusPolicyViolation, convertToNhooyrStatusCodes(wsconnadapter.PolicyViolation))
	require.Equal(suite.T(), websocket.StatusMessageTooBig, convertToNhooyrStatusCodes(wsconnadapter.MessageTooBig))
	require.Equal(suite.T(), websocket.StatusMandatoryExtension, convertToNhooyrStatusCodes(wsconnadapter.MandatoryExtension))
	require.Equal(suite.T(), websocket.StatusInternalError, convertToNhooyrStatusCodes(wsconnadapter.InternalError))
	require.Equal(suite.T(), websocket.StatusTLSHandshake, convertToNhooyrStatusCodes(wsconnadapter.TLSHandshake))
}

// Test status code helper functions.
func (suite *NhooyrWebsocketConnectionAdapterTestSuite) TestMsgTypeConverters() {
	// Test converting to nhoooyr codes
	require.Equal(suite.T(), websocket.MessageText, convertToNhooyrMsgTypes(wsconnadapter.Text))
	require.Equal(suite.T(), websocket.MessageBinary, convertToNhooyrMsgTypes(wsconnadapter.Binary))
	require.Equal(suite.T(), websocket.MessageBinary, convertToNhooyrMsgTypes(-1))
	// Test converting from nhoooyr codes
	require.Equal(suite.T(), wsconnadapter.Text, convertFromNhooyrMsgTypes(websocket.MessageText))
	require.Equal(suite.T(), wsconnadapter.Binary, convertFromNhooyrMsgTypes(websocket.MessageBinary))
	require.Equal(suite.T(), wsconnadapter.Binary, convertFromNhooyrMsgTypes(-1))
}

// # Description
//
// Test if connection is automatically closed when provided context is canceled.
func (suite *NhooyrWebsocketConnectionAdapterTestSuite) TestClosureOnCtxCanceled() {
	// Create adapter
	adapter := NewNhooyrWebsocketConnectionAdapter(nil)
	// Create cancelable context
	ctx, cancel := context.WithCancel(context.Background())
	// Connect to the server
	_, err := adapter.Dial(ctx, *suite.srvUrl)
	require.NoError(suite.T(), err)
	// Ping
	native := adapter.GetUnderlyingWebsocketConnection().(*websocket.Conn)
	native.CloseRead(ctx)
	err = adapter.Ping(ctx)
	require.NoError(suite.T(), err)
	// Cancel context
	cancel()
	// Ping agian and witness connection is closed
	err = adapter.Ping(ctx)
	require.Error(suite.T(), err)
}
