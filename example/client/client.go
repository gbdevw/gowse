package client

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gbdevw/gowsclient/demowsserver"
	"github.com/gbdevw/gowsclient/wscengine/wsadapters"
	"github.com/gbdevw/gowsclient/wscengine/wsclient"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// Example impl. of a websocket client which uses a logger as sink for received messages.
type ExampleClientImpl struct {
	// Number of processed messages
	processedMsgCount int64
	// Logger used by the client implementation to print received messages
	logger *zap.Logger
	// Tracer used by the client implementation to trace received messages
	tracer trace.Tracer
}

// # Description
//
// Factory which creates a new ExampleClientImpl.
//
// # Inputs
//
//   - logger: Logger used to print received messages and events. Use a Nop logger if nil.
//   - tracerProvider: Tracer provider to use to get a tracer to instrument client. Use global tracer provider if nil.
//
// # Returns
//
// A new ExampleClientImpl.
func NewExampleClientImpl(logger *zap.Logger, tracerProvider trace.TracerProvider) *ExampleClientImpl {
	if logger == nil {
		// Use Nop logger if nil is provided
		logger = zap.NewNop()
	}
	if tracerProvider == nil {
		// Use global tracer provider if nil is provided
		tracerProvider = otel.GetTracerProvider()
	}
	// Build and return client
	return &ExampleClientImpl{
		logger: logger,
		tracer: tracerProvider.Tracer("wsclient.example"),
	}
}

func (client *ExampleClientImpl) OnOpen(
	ctx context.Context,
	conn wsadapters.WebsocketConnectionAdapterInterface,
	readMutex *sync.Mutex,
	exit context.CancelFunc,
	restarting bool) error {
	// Start a new span
	_, span := client.tracer.Start(ctx, "wsclient.example.on_open", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()
	defer span.SetStatus(codes.Ok, codes.Ok.String())
	// Log
	client.logger.Info("OnOpen callback called", zap.Bool("restarting", restarting))
	// Subscribe heartbeat
	req := demowsserver.SubscribeRequest{
		Request: demowsserver.Request{
			Message: demowsserver.Message{
				MsgType: demowsserver.MSG_TYPE_SUBSCRIBE_REQUEST,
			},
			ReqId: fmt.Sprint(client.processedMsgCount),
		},
		Topic: demowsserver.MSG_TYPE_HEARTBEAT,
	}
	// Marshal request to JSON
	raw, err := json.Marshal(req)
	if err != nil {
		return handleError(span, err, codes.Error, codes.Error.String())
	}
	// Send request
	err = conn.Write(ctx, wsadapters.Text, raw)
	if err != nil {
		return handleError(span, err, codes.Error, codes.Error.String())
	}
	// Create a context with deadline to wait max 60 seconds for the subscribe response
	deadlineCtx, cancel := context.WithDeadline(ctx, time.Now().Add(60*time.Second))
	defer cancel()
	// Read messages until either a response for subscription is received or deadline is exceeded
	for {
		select {
		case <-deadlineCtx.Done():
			// Trace deadline exceeded and return error
			// -> could not subscribe -> OnOpen return an error -> Websocket engine will stop
			return handleError(span, deadlineCtx.Err(), codes.Error, codes.Error.String())
		default:
			// Read messages from server
			msgType, msg, err := conn.Read(ctx)
			if err != nil {
				return handleError(span, err, codes.Error, codes.Error.String())
			}
			// Unmarshal to response
			response := new(demowsserver.Response)
			err = json.Unmarshal(msg, response)
			if err != nil {
				return handleError(span, err, codes.Error, codes.Error.String())
			}
			// Check the response type
			switch response.MsgType {
			case demowsserver.MSG_TYPE_SUBSCRIBE_RESPONSE:
				// Unmarshal message to the right definitive type
				subscribeResp := new(demowsserver.SubscribeResponse)
				err = json.Unmarshal(msg, response)
				if err != nil {
					return handleError(span, err, codes.Error, codes.Error.String())
				}
				// Check response status
				if subscribeResp.Status == demowsserver.RESPONSE_STATUS_OK {
					return nil
				} else {
					// Craft error from server response error data amd return it
					respErr := fmt.Errorf("%s - %s", subscribeResp.Err.Code, subscribeResp.Err.Message)
					return handleError(span, fmt.Errorf("failed to subscribe to heartbeat publication: %w", respErr), codes.Error, codes.Error.String())
				}
			default:
				// Let's call OnMessage to handle unwanted messages ;)
				client.OnMessage(ctx, conn, readMutex, func() {}, exit, "ONOPEN", msgType, msg)
			}
		}
	}
}

func (client *ExampleClientImpl) OnMessage(
	ctx context.Context,
	conn wsadapters.WebsocketConnectionAdapterInterface,
	readMutex *sync.Mutex,
	restart context.CancelFunc,
	exit context.CancelFunc,
	sessionId string,
	msgType wsadapters.MessageType,
	msg []byte) {
	// Start a new span
	_, span := client.tracer.Start(ctx, "wsclient.example.on_message",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.Int("message_type", int(msgType)),
			attribute.String("message", string(msg)),
		))
	defer span.End()
	defer span.SetStatus(codes.Ok, codes.Ok.String())
	// Log
	client.logger.Info("OnMessage callback called", zap.Int("message_type", int(msgType)), zap.String("message", string(msg)))
	// Increase nmber of processed messages
	client.processedMsgCount = client.processedMsgCount + 1
	// Every 3 processed message, send a echo request
	if client.processedMsgCount%3 == 0 {
		// Craft echo request
		req := demowsserver.EchoRequest{
			Request: demowsserver.Request{
				Message: demowsserver.Message{
					MsgType: demowsserver.MSG_TYPE_ECHO_REQUEST,
				},
				ReqId: fmt.Sprint(client.processedMsgCount),
			},
			Echo: fmt.Sprintf("I have processed %d messages", client.processedMsgCount),
			Err:  false,
		}
		// Serialize echo request
		raw, err := json.Marshal(req)
		if err != nil {
			// Handle error and exit
			handleError(span, err, codes.Error, codes.Error.String())
			return
		}
		// Send echo request
		err = conn.Write(ctx, wsadapters.Text, raw)
		if err != nil {
			handleError(span, err, codes.Error, codes.Error.String())
		}
	}
}

func (client *ExampleClientImpl) OnReadError(
	ctx context.Context,
	conn wsadapters.WebsocketConnectionAdapterInterface,
	readMutex *sync.Mutex,
	restart context.CancelFunc,
	exit context.CancelFunc,
	err error) {
	// Start a new span
	_, span := client.tracer.Start(ctx, "wsclient.example.on_read_error", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()
	defer span.SetStatus(codes.Ok, codes.Ok.String())
	// Record error
	span.RecordError(err)
	// Log
	client.logger.Error("OnReadError callback called", zap.Error(err))
}

func (client *ExampleClientImpl) OnClose(
	ctx context.Context,
	conn wsadapters.WebsocketConnectionAdapterInterface,
	readMutex *sync.Mutex,
	closeMessage *wsclient.CloseMessageDetails) *wsclient.CloseMessageDetails {
	// Start a new span
	_, span := client.tracer.Start(ctx, "wsclient.example.on_close", trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(attribute.String("close", fmt.Sprintf("%v", closeMessage))))
	defer span.End()
	defer span.SetStatus(codes.Ok, codes.Ok.String())
	// Log
	client.logger.Warn("OnClose callback called", zap.Any("close", closeMessage))
	return nil
}

func (client *ExampleClientImpl) OnCloseError(
	ctx context.Context,
	err error) {
	// Start a new span
	_, span := client.tracer.Start(ctx, "wsclient.example.on_close_error", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()
	defer span.SetStatus(codes.Ok, codes.Ok.String())
	// Record error
	span.RecordError(err)
	// Log
	client.logger.Error("OnCloseError callback called", zap.Error(err))
}

func (client *ExampleClientImpl) OnRestartError(
	ctx context.Context,
	exit context.CancelFunc,
	err error,
	retryCount int) {
	// Start a new span
	_, span := client.tracer.Start(ctx, "wsclient.example.on_restart_error",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(attribute.Int("retries", retryCount)))
	defer span.End()
	defer span.SetStatus(codes.Ok, codes.Ok.String())
	// Record error
	span.RecordError(err)
	// Log
	client.logger.Error("OnRestartError callback called", zap.Error(err), zap.Int("retires", retryCount))
}

// # Description
//
// Helper function to record an error and set a span status in one instruction. The input
// error is returned as-is by the function.
//
// # Motivation
//
// The goal of this function is to reduce error handling blocks like this:
//
//	if err != nil {
//			span.RecordError(err)
//			span.SetStatus(codes.Error, codes.Error.String())
//			return err
//		}
//
// To this:
//
//	if err != nil {
//			return handleError(span, err, codes.Error, codes.Error.String())
//	}
func handleError(span trace.Span, err error, code codes.Code, description string) error {
	span.RecordError(err)
	span.SetStatus(code, description)
	return err
}
