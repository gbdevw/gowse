package client

import (
	"context"
	"fmt"
	"sync"

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
	return nil
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
