package wscengine

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/gbdevw/gowse/echowsserver"
	"github.com/gbdevw/gowse/wscengine/wsadapters"
	wsadapternhooyr "github.com/gbdevw/gowse/wscengine/wsadapters/nhooyr"
	"github.com/gbdevw/gowse/wscengine/wsclient"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

/*************************************************************************************************/
/* TEST SUITES                                                                                   */
/*************************************************************************************************/

// Test suite used for WebsocketEngine unit tests
type WebsocketEngineUnitTestSuite struct {
	suite.Suite
}

// Run WebsocketEngineUnitTestSuite test suite
func TestWebsocketEngineUnitTestSuite(t *testing.T) {
	suite.Run(t, new(WebsocketEngineUnitTestSuite))
}

// Test suite used to test WebsocketEngine against a live websocket server
type WebsocketEngineIntegrationTestSuite struct {
	suite.Suite
	srv    *echowsserver.EchoWebsocketServer
	srvUrl *url.URL
}

// Run WebsocketEngineIntegrationTestSuite test suite
func TestWebsocketEngineIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(WebsocketEngineIntegrationTestSuite))
}

// WebsocketEngineIntegrationTestSuite - Before all tests
func (suite *WebsocketEngineIntegrationTestSuite) SetupSuite() {
	// Create server
	host := "localhost:8081"
	srv := echowsserver.NewEchoWebsocketServer(&http.Server{Addr: host}, log.New(io.Discard, "", log.Flags()))
	require.NotNil(suite.T(), srv)
	// Start server
	err := srv.Start()
	require.NoError(suite.T(), err)
	// Assign server to suite
	suite.srv = srv
	srvUrl, err := url.Parse("ws://" + host)
	require.NoError(suite.T(), err)
	suite.srvUrl = srvUrl
}

// WebsocketEngineIntegrationTestSuite - Before all tests
func (suite *WebsocketEngineIntegrationTestSuite) TearDownSuite() {
	// Stop server
	suite.srv.Stop()
}

/*************************************************************************************************/
/* UNIT TESTS                                                                                    */
/*************************************************************************************************/

// # Description
//
// Test will ensure engine properly handle the case when a mocked connection conn.Read returns
// errors that are not a EOF or close errors. Engine is expected to call OnReadError callback and
// then continue to loop on a second error. OnReadError is expected to be called and to use the
// provided cancel to stop the engine to shutdown.
//
// Engine is then expected to shutdown, call OnClose without a close message and close the
// mocked connection with the close message returned by mocked OnClose callback.
//
// Test will succeed if:
//   - Mocked connection returns an error which cause OnReadError callback to be called.
//   - Engine loops and call a second time conn.Read which returns an error.
//   - OnReadError is called by the engine and provided exit function is called.
//   - Engine starts its shutdown phase and call OnClose callback without a close message.
//   - OnClose is called and return a close message
//   - conn.Close is called with the close message returned by OnClose.
//   - Engine finally stops
func (suite *WebsocketEngineUnitTestSuite) TestOnReadErrorHandlingAndThenShutdown() {
	// Create context with cancel function
	ctx, cancel := context.WithCancel(context.Background())
	// Expected error returned from conn.Read
	connReadErr := fmt.Errorf("mocked conn.Read error")
	// Expected OnClose returned close message
	closeMsg := wsclient.CloseMessageDetails{
		CloseReason:  wsadapters.GoingAway,
		CloseMessage: "client shutdown the connection after error",
	}
	// Create & configure connection mock
	connMock := wsadapters.NewWebsocketConnectionAdapterInterfaceMock()
	connMock.
		On("Read", mock.Anything).Return(-1, []byte{}, connReadErr).
		On("Close", mock.Anything, closeMsg.CloseReason, closeMsg.CloseMessage).Return(nil)
	// Configure OnReadError mock which does nothing on first call and
	// call the provided engine exit function on second call
	mockWsClient := wsclient.NewWebsocketClientMock()
	mockWsClient.
		On("OnClose",
			mock.Anything,
			mock.Anything,
			mock.Anything,
			(*wsclient.CloseMessageDetails)(nil)).Return(&closeMsg).
		On("OnReadError",
			mock.Anything,
			mock.Anything,
			mock.Anything,
			mock.Anything,
			mock.Anything,
			connReadErr).Return().Once().
		On("OnReadError",
			mock.Anything,
			mock.Anything,
			mock.Anything,
			mock.Anything,
			mock.Anything,
			connReadErr).Return().Run(func(args mock.Arguments) {
		cancel()
	})
	// Create engine with mocks
	srvUrl, err := url.Parse("ws://localhost")
	require.NoError(suite.T(), err)
	opts := NewWebsocketEngineConfigurationOptions().
		WithAutoReconnect(false)
	engine, err := NewWebsocketEngine(srvUrl, connMock, mockWsClient, opts, nil)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), engine)

	// Call engine.runWebsocketEngine to start the test
	engine.runEngine(
		ctx,
		cancel,
		cancel,
		connMock,
		&sync.Once{},
		uuid.New().String(),
		uuid.New().String(),
	)
	// Check on mocks
	connMock.AssertNumberOfCalls(suite.T(), "Read", 2)
	connMock.AssertNumberOfCalls(suite.T(), "Close", 1)
	mockWsClient.AssertNumberOfCalls(suite.T(), "OnClose", 1)
	mockWsClient.AssertNumberOfCalls(suite.T(), "OnReadError", 2)
	mockWsClient.AssertNumberOfCalls(suite.T(), "OnCloseError", 0)
}

// # Description
//
// Test will ensure an engine goroutine can trigger engine shutdown once it has acquired read
// mutex.
//
// Note: this is only to hit 100% coverage at time of test creation ...
func (suite *WebsocketEngineUnitTestSuite) TestShutdownAfterReadMutexUnlock() {
	// Create context with cancel function
	ctx, cancel := context.WithCancel(context.Background())
	// Expected OnClose returned close message
	closeMsg := wsclient.CloseMessageDetails{
		CloseReason:  wsadapters.GoingAway,
		CloseMessage: "client shutdown the connection after error",
	}
	// Create & configure connection mock
	connMock := wsadapters.NewWebsocketConnectionAdapterInterfaceMock()
	connMock.
		On("Close", mock.Anything, closeMsg.CloseReason, mock.Anything).Return(nil)
	// Configure OnReadError mock which does nothing on first call and
	// call the provided engine exit function on second call
	mockWsClient := wsclient.NewWebsocketClientMock()
	mockWsClient.
		On("OnClose",
			mock.Anything,
			mock.Anything,
			mock.Anything,
			(*wsclient.CloseMessageDetails)(nil)).Return(&closeMsg)
	// Create engine with mocks
	srvUrl, err := url.Parse("ws://localhost")
	require.NoError(suite.T(), err)
	opts := NewWebsocketEngineConfigurationOptions().
		WithAutoReconnect(false)
	engine, err := NewWebsocketEngine(srvUrl, connMock, mockWsClient, opts, nil)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), engine)
	// Lock read mutex
	engine.GetReadMutex().Lock()
	// Create a goroutine which will execute runEngine
	go engine.runEngine(
		ctx,
		cancel,
		cancel,
		connMock,
		&sync.Once{},
		uuid.New().String(),
		uuid.New().String(),
	)
	// Cancel context to force goroutine to exit
	cancel()
	// Unlock read mutex to let goroutine acquire it and exit
	engine.GetReadMutex().Unlock()
	// Read on stop channel to know when goroutine has finished
	<-engine.stoppedChannel
	// Check on mocks
	connMock.AssertNumberOfCalls(suite.T(), "Close", 0)
	mockWsClient.AssertNumberOfCalls(suite.T(), "OnClose", 1)
	mockWsClient.AssertNumberOfCalls(suite.T(), "OnCloseError", 0)
}

// # Description
//
// Test will ensure Start method returns an error when callled with a canceled context.
func (suite *WebsocketEngineUnitTestSuite) TestStartWithCanceledCtx() {
	// Create context with cancel function
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	// Create valid URL
	srvUrl, err := url.Parse("ws://localhost")
	require.NoError(suite.T(), err)
	// Create Conn& Client mocks
	connMock := wsadapters.NewWebsocketConnectionAdapterInterfaceMock()
	clientMock := wsclient.NewWebsocketClientMock()
	// Create engine
	engine, err := NewWebsocketEngine(srvUrl, connMock, clientMock, nil, nil)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), engine)
	// Call Start with canceled context
	err = engine.Start(ctx)
	// Verify error wraps context error
	require.ErrorIs(suite.T(), err, ctx.Err())
}

// # Description
//
// Test will ensure Start method returns an error when a timeout occurs during engine startup.
func (suite *WebsocketEngineUnitTestSuite) TestStartWithTimeoutError() {
	// Configure OnOpenTimeoutMs
	timeoutMs := 1000
	opts := NewWebsocketEngineConfigurationOptions().
		WithOnOpenTimeoutMs(int64(timeoutMs))
	// Create valid URL
	srvUrl, err := url.Parse("ws://localhost")
	require.NoError(suite.T(), err)
	// Create Conn & Client mocks
	connMock := wsadapters.NewWebsocketConnectionAdapterInterfaceMock()
	connMock.On("Close", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	clientMock := wsclient.NewWebsocketClientMock()
	// Configure connMock Dial to wait enough time to trigger the timeout
	connMock.On("Dial", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			time.Sleep(time.Duration(timeoutMs*2) * time.Millisecond)
		}).
		Return((*http.Response)(nil), fmt.Errorf("will error"))
	// Create engine
	engine, err := NewWebsocketEngine(srvUrl, connMock, clientMock, opts, nil)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), engine)
	// Call Start
	err = engine.Start(context.Background())
	// Verify error wraps a EngineStartError
	require.ErrorAs(suite.T(), err, &EngineStartError{})
	// Check on mock
	connMock.AssertNumberOfCalls(suite.T(), "Dial", 1)
}

// # Description
//
// Test will ensure Start method returns the error returned by OnOpen when OnOpen returns an error
// during engine startup. Test will also ensure engine close the connection any case OnOpen returns
// an error.
func (suite *WebsocketEngineUnitTestSuite) TestStartWithOnOpenError() {
	// Expected OnOpen error
	expectedErr := fmt.Errorf("onOpen callback error")
	// Create valid URL
	srvUrl, err := url.Parse("ws://localhost")
	require.NoError(suite.T(), err)
	// Create Conn & Client mocks
	connMock := wsadapters.NewWebsocketConnectionAdapterInterfaceMock()
	clientMock := wsclient.NewWebsocketClientMock()
	// Configure connMock Dial and Close to succeed
	connMock.
		On("Dial", mock.Anything, mock.Anything).Return((*http.Response)(nil), nil).
		On("Close", mock.Anything, wsadapters.GoingAway, mock.Anything).Return(nil)
	// Configure client mock OnOpen to return an error when called
	clientMock.On("OnOpen", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, false).
		Return(expectedErr)
	// Create engine
	engine, err := NewWebsocketEngine(srvUrl, connMock, clientMock, nil, nil)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), engine)
	// Call Start
	err = engine.Start(context.Background())
	require.ErrorIs(suite.T(), err, expectedErr)
	// Verify mocks
	connMock.AssertNumberOfCalls(suite.T(), "Dial", 1)
	connMock.AssertNumberOfCalls(suite.T(), "Close", 1)
	clientMock.AssertNumberOfCalls(suite.T(), "OnOpen", 1)
}

// # Description
//
// Test startEngine method when called with a canceled context. Method is expected to return
// an EngineStartError through a provided channel.
func (suite *WebsocketEngineUnitTestSuite) TestStartEngineWithCanceledCtx() {
	// Create canceled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	// Create valid URL
	srvUrl, err := url.Parse("ws://localhost")
	require.NoError(suite.T(), err)
	// Create Conn & Client mocks
	connMock := wsadapters.NewWebsocketConnectionAdapterInterfaceMock()
	clientMock := wsclient.NewWebsocketClientMock()
	// Create engine
	engine, err := NewWebsocketEngine(srvUrl, connMock, clientMock, nil, nil)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), engine)
	// Create startupChannel
	startupChannel := make(chan error, 1)
	// Call startEngine
	engine.startEngine(ctx, false, startupChannel, func() {})
	// Read error from channel
	select {
	case err := <-startupChannel:
		// Check error is a EngineStartError which contain the ctx.Err()
		require.ErrorAs(suite.T(), err, &EngineStartError{})
		require.ErrorIs(suite.T(), err, ctx.Err())
	default:
		suite.FailNow("an error should have been read on the channel")
	}
}

// # Description
//
// Test will ensure startEngine method returns an error in the provided channel in case the engine
// has already its started flag set and engine is not restarting.
func (suite *WebsocketEngineUnitTestSuite) TestStartEngineWithEngineAlreadyStarted() {
	// Create valid URL
	srvUrl, err := url.Parse("ws://localhost")
	require.NoError(suite.T(), err)
	// Create Conn & Client mocks
	connMock := wsadapters.NewWebsocketConnectionAdapterInterfaceMock()
	clientMock := wsclient.NewWebsocketClientMock()
	// Create engine
	engine, err := NewWebsocketEngine(srvUrl, connMock, clientMock, nil, nil)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), engine)
	// Set engine started flag
	engine.started = true
	// Create startupChannel
	startupChannel := make(chan error, 1)
	// Call startEngine
	engine.startEngine(context.Background(), false, startupChannel, func() {})
	// Read error from channel
	select {
	case err := <-startupChannel:
		// Check error is a EngineStartError
		require.ErrorAs(suite.T(), err, &EngineStartError{})
	default:
		suite.FailNow("an error should have been read on the channel")
	}
}

// # Description
//
// Test will ensure Stop method returns an error when called while engine has not started.
func (suite *WebsocketEngineUnitTestSuite) TestStopWithEngineNotStarted() {
	// Create valid URL
	srvUrl, err := url.Parse("ws://localhost")
	require.NoError(suite.T(), err)
	// Create Conn & Client mocks
	connMock := wsadapters.NewWebsocketConnectionAdapterInterfaceMock()
	clientMock := wsclient.NewWebsocketClientMock()
	// Create engine
	engine, err := NewWebsocketEngine(srvUrl, connMock, clientMock, nil, nil)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), engine)
	// Call stop
	err = engine.Stop(context.Background())
	require.Error(suite.T(), err)
}

// # Description
//
// Test will ensure restartEngine calls OnRestartError and loop when engine fails to restart.
// OnRestartError callback will call the engine exit function to stop the engine.
func (suite *WebsocketEngineUnitTestSuite) TestRestartEngineErrorPath() {
	// Timeout
	timeoutMs := 1000
	// Options with timeout
	opts := NewWebsocketEngineConfigurationOptions().
		WithOnOpenTimeoutMs(int64(timeoutMs))
	// Create cancelable context
	ctx, cancel := context.WithCancel(context.Background())
	// Create valid URL
	srvUrl, err := url.Parse("ws://localhost")
	require.NoError(suite.T(), err)
	// Create Conn & Client mocks
	connMock := wsadapters.NewWebsocketConnectionAdapterInterfaceMock()
	clientMock := wsclient.NewWebsocketClientMock()
	// Configure conn.Dial to fail to fail engine restart
	dialErr := fmt.Errorf("error on dial call")
	expectedErr := EngineStartError{Err: dialErr}
	connMock.
		// First call will timeout
		On("Dial", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		time.Sleep(time.Duration(2*timeoutMs) * time.Millisecond)
	}).
		Return((*http.Response)(nil), nil).Once().
		// Second call will return an error
		On("Dial", mock.Anything, mock.Anything).Return((*http.Response)(nil), dialErr).
		// Handle close
		On("Close", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	// Configure client OnRestartError to call cancel function
	clientMock.
		// First call does nothing
		On("OnRestartError", mock.Anything, mock.Anything, mock.Anything, 0).
		// Second call interrupt retry loop with exit function
		On("OnRestartError", mock.Anything, mock.Anything, expectedErr, 1).
		Run(func(args mock.Arguments) {
			cancel()
		})
	// Create engine
	engine, err := NewWebsocketEngine(srvUrl, connMock, clientMock, opts, nil)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), engine)
	// Set started flag, ctx and exit function
	engine.started = true
	engine.engineCtx = ctx
	engine.engineStopFunc = cancel
	// Call restartEngine
	engine.restartEngine(engine.engineCtx, engine.stoppedChannel, engine.engineStopFunc)
	// Verify engine has stopped
	select {
	case <-engine.stoppedChannel:
		// Verify mocks
		connMock.AssertNumberOfCalls(suite.T(), "Dial", 2)
		connMock.AssertNumberOfCalls(suite.T(), "Close", 1)
		clientMock.AssertNumberOfCalls(suite.T(), "OnRestartError", 2)
	default:
		suite.FailNow("somehting should have been read on stopped channel")
	}
}

// # Description
//
// Test will ensure GetReadMutex returns the engine read mutex.
func (suite *WebsocketEngineUnitTestSuite) TestGetReadMutex() {
	// Create valid URL
	srvUrl, err := url.Parse("ws://localhost")
	require.NoError(suite.T(), err)
	// Create Conn & Client mocks
	connMock := wsadapters.NewWebsocketConnectionAdapterInterfaceMock()
	clientMock := wsclient.NewWebsocketClientMock()
	// Create engine
	engine, err := NewWebsocketEngine(srvUrl, connMock, clientMock, nil, nil)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), engine)
	// call GetReadMutex
	readMu := engine.GetReadMutex()
	require.Equal(suite.T(), engine.readMutex, readMu)
}

// # Description
//
// Test engine factory function with invalid inputs. Factory is expected to return an error.
//
// Test will succeed if:
//   - Factory returns an error when used with a nil URL
//   - Factory returns an error when used with a nil connection adapter
//   - Factory returns an error when used with a nil websocket client
//   - Factory returns an error when used with invalid options
func (suite *WebsocketEngineUnitTestSuite) TestEngineFactoryInvalidParameters() {
	// Create valid URL
	srvUrl, err := url.Parse("ws://localhost")
	require.NoError(suite.T(), err)
	// Create Conn& Client mocks
	connMock := wsadapters.NewWebsocketConnectionAdapterInterfaceMock()
	clientMock := wsclient.NewWebsocketClientMock()
	// Create invalid options
	invalidOpts := NewWebsocketEngineConfigurationOptions().
		WithReaderRoutinesCount(-1)
	require.Error(suite.T(), Validate(invalidOpts))
	// Call factory with nil URL
	engine, err := NewWebsocketEngine(nil, connMock, clientMock, nil, nil)
	require.Error(suite.T(), err)
	require.Nil(suite.T(), engine)
	// Call factory with nil conn
	engine, err = NewWebsocketEngine(srvUrl, nil, clientMock, nil, nil)
	require.Error(suite.T(), err)
	require.Nil(suite.T(), engine)
	// Call factory with nil client
	engine, err = NewWebsocketEngine(srvUrl, connMock, nil, nil, nil)
	require.Error(suite.T(), err)
	require.Nil(suite.T(), engine)
	// Call factory with invalid options
	engine, err = NewWebsocketEngine(srvUrl, connMock, clientMock, invalidOpts, nil)
	require.Error(suite.T(), err)
	require.Equal(suite.T(), Validate(invalidOpts).Error(), err.Error())
	require.Nil(suite.T(), engine)
}

/*************************************************************************************************/
/* INTEGRATION TESTS                                                                             */
/*************************************************************************************************/

// # Description
//
// Test will ensure websocket engine can Start, open a connection and then Stop.
//
// Test will succeed if:
//   - Engine can start, connect to the server and call OnOpen callback
//   - Engine connection can be used to send a ping to the websocket server
//   - Engine can stop and call OnClose callback
//   - Engine connection is closed once engin stop completes
func (suite *WebsocketEngineIntegrationTestSuite) TestOnOpenAndOnClose() {
	// Create websocket client mock
	wsClientMock := wsclient.NewWebsocketClientMock()
	// Configure mock
	wsClientMock.On("OnOpen", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).
		On("OnClose", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).
		On("OnCloseError", mock.Anything, mock.Anything).Return(nil)
	// Create engine
	conn := wsadapternhooyr.NewNhooyrWebsocketConnectionAdapter(nil)
	engine, err := NewWebsocketEngine(suite.srvUrl, conn, wsClientMock, nil, nil)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), engine)
	// Start the engine
	count := 0
	retryLimit := 3
	for count < 3 {
		err = engine.Start(context.Background())
		if err != nil {
			count = count + 1
			time.Sleep(2 * time.Second)
		} else {
			break
		}
	}
	require.Less(suite.T(), count, retryLimit)
	// Use connection to ping server to ensure connection is up
	timeoutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	err = conn.Ping(timeoutCtx)
	cancel()
	require.NoError(suite.T(), err)
	// Stop the engine
	err = engine.Stop(context.Background())
	require.NoError(suite.T(), err)
	// Reping the server and witness close
	timeoutCtx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	err = conn.Ping(timeoutCtx)
	cancel()
	require.Error(suite.T(), err)
	// Check mock OnOpen and OnClose were called
	wsClientMock.AssertNumberOfCalls(suite.T(), "OnOpen", 1)
	wsClientMock.AssertNumberOfCalls(suite.T(), "OnClose", 1)
	wsClientMock.AssertNumberOfCalls(suite.T(), "OnReadError", 0)
	wsClientMock.AssertNumberOfCalls(suite.T(), "OnCloseError", 0)
	wsClientMock.AssertNumberOfCalls(suite.T(), "OnMessage", 0)
	wsClientMock.AssertNumberOfCalls(suite.T(), "OnRestartError", 0)
}

// # Description
//
// Test will ensure websocket engine can Start, open a connection, receive some echo messages
// and then Stop.
//
// Test will succeed if:
//   - Engine can start, connect to the server and call OnOpen callback
//   - Engine connection can be used to send a ping to the websocket server
//   - Engine connection can be used to send 4 Echo requests to the server
//   - Engine can stop and call OnClose callback
//   - Engine connection is closed once engine stop completes
func (suite *WebsocketEngineIntegrationTestSuite) TestClientSession() {
	// Create websocket client mock
	wsClientMock := wsclient.NewWebsocketClientMock()
	// Configure mock
	wsClientMock.On("OnOpen", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).
		On("OnClose", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).
		On("OnMessage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		On("OnCloseError", mock.Anything, mock.Anything)
	// Create engine
	conn := wsadapternhooyr.NewNhooyrWebsocketConnectionAdapter(nil)
	engine, err := NewWebsocketEngine(suite.srvUrl, conn, wsClientMock, nil, nil)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), engine)
	// Start the engine
	count := 0
	retryLimit := 3
	for count = 0; count < retryLimit; count++ {
		err = engine.Start(context.Background())
		if err != nil {
			time.Sleep(2 * time.Second)
		} else {
			break
		}
	}
	require.Less(suite.T(), count, retryLimit)
	// Use connection to ping server to ensure connection is up
	timeoutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	err = conn.Ping(timeoutCtx)
	cancel()
	require.NoError(suite.T(), err)
	// Send 4 echo requests
	expectedEcho := "hello"
	req1 := fmt.Sprintf(`{"type":"echo_request", "echo": "%s"}`, expectedEcho)
	err = conn.Write(context.Background(), wsadapters.Text, []byte(req1))
	require.NoError(suite.T(), err)
	err = conn.Write(context.Background(), wsadapters.Text, []byte(req1))
	require.NoError(suite.T(), err)
	err = conn.Write(context.Background(), wsadapters.Text, []byte(req1))
	require.NoError(suite.T(), err)
	err = conn.Write(context.Background(), wsadapters.Text, []byte(req1))
	require.NoError(suite.T(), err)
	// Sleep
	time.Sleep(5 * time.Second)
	// Stop the engine
	err = engine.Stop(context.Background())
	require.NoError(suite.T(), err)
	// Reping the server and witness close
	timeoutCtx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	err = conn.Ping(timeoutCtx)
	cancel()
	require.Error(suite.T(), err)
	// Check mock OnOpen and OnClose were called
	wsClientMock.AssertNumberOfCalls(suite.T(), "OnOpen", 1)
	wsClientMock.AssertNumberOfCalls(suite.T(), "OnClose", 1)
	wsClientMock.AssertNumberOfCalls(suite.T(), "OnReadError", 0)
	wsClientMock.AssertNumberOfCalls(suite.T(), "OnCloseError", 0)
	wsClientMock.AssertNumberOfCalls(suite.T(), "OnMessage", 4)
	wsClientMock.AssertNumberOfCalls(suite.T(), "OnRestartError", 0)
	// Verify OnMessage calls
	rcv, ok := wsClientMock.Calls[1].Arguments.Get(7).([]byte)
	require.True(suite.T(), ok)
	require.Contains(suite.T(), string(rcv), expectedEcho)
	rcv, ok = wsClientMock.Calls[2].Arguments.Get(7).([]byte)
	require.True(suite.T(), ok)
	require.Contains(suite.T(), string(rcv), expectedEcho)
	rcv, ok = wsClientMock.Calls[3].Arguments.Get(7).([]byte)
	require.True(suite.T(), ok)
	require.Contains(suite.T(), string(rcv), expectedEcho)
	rcv, ok = wsClientMock.Calls[4].Arguments.Get(7).([]byte)
	require.True(suite.T(), ok)
	require.Contains(suite.T(), string(rcv), expectedEcho)
}

// # Description
//
// Test will ensure websocket engine can reopen a connection when established connection is closed by server.
// Test will also ensure OnClose callback is called in such a case.
//
// Test will succeed if:
//   - Engine can start, connect to the server and call OnOpen callback
//   - Engine connection can be used to send a ping to the websocket server
//   - Engine calls OnClose callback when server closes client connection with a close message
//   - Engine reconnect to the server and call OnOpen callback again
//   - Engine can stop and call OnClose callback
func (suite *WebsocketEngineIntegrationTestSuite) TestEngineAutoReconnect() {
	// Create websocket client mock
	wsClientMock := wsclient.NewWebsocketClientMock()
	// Configure mock
	wsClientMock.On("OnOpen", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).
		On("OnClose", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).
		On("OnCloseError", mock.Anything, mock.Anything)
	// Create engine
	conn := wsadapternhooyr.NewNhooyrWebsocketConnectionAdapter(nil)
	engine, err := NewWebsocketEngine(suite.srvUrl, conn, wsClientMock, nil, nil)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), engine)
	// Start the engine
	count := 0
	retryLimit := 3
	for count < 3 {
		err = engine.Start(context.Background())
		if err != nil {
			count = count + 1
			time.Sleep(2 * time.Second)
		} else {
			break
		}
	}
	require.Less(suite.T(), count, retryLimit)
	// Use connection to ping server to ensure connection is up
	timeoutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	err = conn.Ping(timeoutCtx)
	cancel()
	require.NoError(suite.T(), err)
	// Check engine IsStarted
	require.True(suite.T(), engine.IsStarted())
	// Shutdown the client connection
	conn.Close(context.Background(), wsadapters.GoingAway, "interruption")
	// Wait & check engine is still started
	time.Sleep(3 * time.Second)
	require.True(suite.T(), engine.IsStarted())
	// Use connection to ping server to ensure connection is up
	timeoutCtx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	err = conn.Ping(timeoutCtx)
	cancel()
	require.NoError(suite.T(), err)
	// Stop the engine
	engine.Stop(context.Background())
	// Reping the server and witness close
	timeoutCtx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	err = conn.Ping(timeoutCtx)
	cancel()
	require.Error(suite.T(), err)
	// Check engine state
	require.False(suite.T(), engine.IsStarted())
	// Check mock OnOpen and OnClose were called
	wsClientMock.AssertNumberOfCalls(suite.T(), "OnOpen", 2)
	wsClientMock.Calls[0].Arguments.Assert(suite.T(), mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, false)
	wsClientMock.Calls[2].Arguments.Assert(suite.T(), mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, true)
	wsClientMock.AssertNumberOfCalls(suite.T(), "OnClose", 2)
	require.NotNil(suite.T(), wsClientMock.Calls[1].Arguments.Get(2)) // Must have a close message as server shutdown the connection (eiter with or without a close message)
	require.Nil(suite.T(), wsClientMock.Calls[3].Arguments.Get(3))    // Calling stop = no closeMessage in OnClose callback
	wsClientMock.AssertNumberOfCalls(suite.T(), "OnReadError", 0)
	wsClientMock.AssertNumberOfCalls(suite.T(), "OnCloseError", 0)
	wsClientMock.AssertNumberOfCalls(suite.T(), "OnMessage", 0)
	wsClientMock.AssertNumberOfCalls(suite.T(), "OnRestartError", 0)
}
