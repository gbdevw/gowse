package wsclient

import (
	"context"
	"fmt"
	"sync"

	"github.com/stretchr/testify/mock"
	adapters "gitlab.com/lake42/go-websocket-client/pkg/wsclientengine/adapters"
)

/*****************************************************************************/
/* WEBSOCKET CLIENT MOCK                                                     */
/*****************************************************************************/

// Mock for WebsocketClientInterface. Each callback has its own mock that is used when a callback is called.
type WebsocketClientMock struct {
	mock.Mock
}

// Mocked OnOpen method
func (mock *WebsocketClientMock) OnOpen(
	ctx context.Context,
	conn adapters.WebsocketConnectionAdapterInterface,
	readMutex *sync.Mutex,
	exit context.CancelFunc,
	restarting bool) error {
	// Call mocked method with provided args and return predefined return value if any
	args := mock.Called(ctx, conn, readMutex, exit, restarting)
	return args.Error(0)
}

// Mocked OnMessage method
func (mock *WebsocketClientMock) OnMessage(
	ctx context.Context,
	conn adapters.WebsocketConnectionAdapterInterface,
	readMutex *sync.Mutex,
	restart context.CancelFunc,
	exit context.CancelFunc,
	sessionId string,
	msgType adapters.MessageType,
	msg []byte) {
	// Call mocked method with provided args and return predefined return value if any
	mock.Called(ctx, conn, readMutex, restart, exit, sessionId, msgType, msg)
}

// Mocked OnReadError method
func (mock *WebsocketClientMock) OnReadError(
	ctx context.Context,
	conn adapters.WebsocketConnectionAdapterInterface,
	readMutex *sync.Mutex,
	restart context.CancelFunc,
	exit context.CancelFunc,
	err error) {
	// Call mocked method with provided args and return predefined return value if any
	mock.Called(ctx, conn, readMutex, restart, exit, err)
}

// Mocked OnClose method
func (mock *WebsocketClientMock) OnClose(
	ctx context.Context,
	conn adapters.WebsocketConnectionAdapterInterface,
	readMutex *sync.Mutex,
	closeMessage *CloseMessageDetails) *CloseMessageDetails {
	// Call mocked method with provided args and return predefined return value if any
	args := mock.Called(ctx, conn, readMutex, closeMessage)
	if args.Get(0) != nil {
		// Type assertion
		msg, ok := args.Get(0).(*CloseMessageDetails)
		if ok {
			return msg
		} else {
			panic(fmt.Sprintf("mocked OnClose returned value is not nil or a *CloseMessageDetails. Got %T, %v", msg, msg))
		}
	} else {
		return nil
	}
}

// Mocked OnCloseError method
func (mock *WebsocketClientMock) OnCloseError(
	ctx context.Context,
	err error) {
	// Call mocked method with provided args and return predefined return value if any
	mock.Called(ctx, err)
}

func (mock *WebsocketClientMock) OnRestartError(
	ctx context.Context,
	exit context.CancelFunc,
	err error,
	retryCount int) {
	// Call mocked method with provided args and return predefined return value if any
	mock.Called(ctx, exit, err, retryCount)
}

// Factory for WebsocketClientMock
func NewWebsocketClientMock() *WebsocketClientMock {
	return &WebsocketClientMock{
		Mock: mock.Mock{},
	}
}
