package wscengine

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/gbdevw/gowse/wscengine/wsadapters"
	"github.com/gbdevw/gowse/wscengine/wsclient"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// Package private decorator used to trace user provided callbacks
type websocketClientInstrumentationDecorator struct {
	// Tracer used to instrument code
	tracer trace.Tracer
	// Decorated WebsocketClientInterface implementation
	decorated wsclient.WebsocketClientInterface
}

// # Description
//
// Build and return a new decorator which instrument a provided WebsocketClientInterface implementation.
//
// # Inputs
//
//   - decorated: The WebsocketClientInterface implementation to decorate. Must not be nil.
//   - tracerProvider: Tracer provider used to get a tracer. If nil, global traver provider will be used.
//
// # Returns
//
// A new insturmentation decorator for the provided WebsocketClientInterface implementation or an error
// if decorated is nil.
func NewWebsocketClientInstrumentationDecorator(decorated wsclient.WebsocketClientInterface, tracerProvider trace.TracerProvider) (*websocketClientInstrumentationDecorator, error) {
	if decorated == nil {
		// Return an error if decorated is nil
		return nil, fmt.Errorf("provided decorated is nil")
	}
	if tracerProvider == nil {
		// Use global tracer provider as instead
		tracerProvider = otel.GetTracerProvider()
	}
	// Build and return decorator
	return &websocketClientInstrumentationDecorator{
		decorated: decorated,
		tracer:    tracerProvider.Tracer(pkgName, trace.WithInstrumentationVersion(pkgVersion)),
	}, nil
}

// Instrument decorated.OnOpen call
func (decorator *websocketClientInstrumentationDecorator) OnOpen(
	ctx context.Context,
	resp *http.Response,
	conn wsadapters.WebsocketConnectionAdapterInterface,
	readMutex *sync.Mutex,
	exit context.CancelFunc,
	restarting bool) error {
	// Start a span
	ctx, span := decorator.tracer.Start(ctx, spanEngineOnOpen,
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.Bool(attrRestart, restarting),
		))
	defer span.End()
	// Call decorated.OnOpen, handle and return results
	err := decorator.decorated.OnOpen(ctx, resp, conn, readMutex, exit, restarting)
	return handlePotentialError(err, span)
}

// Instrument decorated.OnMessage call
func (decorator *websocketClientInstrumentationDecorator) OnMessage(
	ctx context.Context,
	conn wsadapters.WebsocketConnectionAdapterInterface,
	readMutex *sync.Mutex,
	restart context.CancelFunc,
	exit context.CancelFunc,
	sessionId string,
	msgType wsadapters.MessageType,
	msg []byte) {
	// Start a span
	ctx, span := decorator.tracer.Start(ctx, spanEngineOnMessage,
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.Int(attrMsgType, int(msgType)),
			attribute.Int(attrMsgLength, len(msg)),
		))
	defer span.End()
	defer span.SetStatus(codes.Ok, codes.Ok.String())
	// Call decorated.OnMessage
	decorator.decorated.OnMessage(ctx, conn, readMutex, restart, exit, sessionId, msgType, msg)
}

// Instrument decorated.OnReadError call
func (decorator *websocketClientInstrumentationDecorator) OnReadError(
	ctx context.Context,
	conn wsadapters.WebsocketConnectionAdapterInterface,
	readMutex *sync.Mutex,
	restart context.CancelFunc,
	exit context.CancelFunc,
	err error) {
	// Start span
	ctx, span := decorator.tracer.Start(ctx, spanEngineOnReadError,
		trace.WithSpanKind(trace.SpanKindInternal))
	defer span.End()
	defer span.SetStatus(codes.Ok, codes.Ok.String())
	// Call decorated.OnReadError
	decorator.decorated.OnReadError(ctx, conn, readMutex, restart, exit, err)
}

// Instument decorated.OnClose call
func (decorator *websocketClientInstrumentationDecorator) OnClose(
	ctx context.Context,
	conn wsadapters.WebsocketConnectionAdapterInterface,
	readMutex *sync.Mutex,
	closeMessage *wsclient.CloseMessageDetails) *wsclient.CloseMessageDetails {
	// Start span
	ctx, span := decorator.tracer.Start(ctx, spanEngineOnClose,
		trace.WithSpanKind(trace.SpanKindInternal))
	defer span.End()
	defer span.SetStatus(codes.Ok, codes.Ok.String())
	// Call decorated.OnClose and return results
	return decorator.decorated.OnClose(ctx, conn, readMutex, closeMessage)
}

// Instrument decorated.OnCloseError call
func (decorator *websocketClientInstrumentationDecorator) OnCloseError(
	ctx context.Context,
	err error) {
	// Start span
	ctx, span := decorator.tracer.Start(ctx, spanEngineOnCloseError,
		trace.WithSpanKind(trace.SpanKindInternal))
	defer span.End()
	defer span.SetStatus(codes.Ok, codes.Ok.String())
	// Call decorated.OnCloseError
	decorator.decorated.OnCloseError(ctx, err)
}

// Instrument decorated.OnRestartError call
func (decorator *websocketClientInstrumentationDecorator) OnRestartError(
	ctx context.Context,
	exit context.CancelFunc,
	err error,
	retryCount int) {
	// Start span
	ctx, span := decorator.tracer.Start(ctx, spanEngineOnRestartError,
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.Int(attrRetryCount, retryCount),
		))
	defer span.End()
	defer span.SetStatus(codes.Ok, codes.Ok.String())
	// Call decorated.OnCloseError
	decorator.decorated.OnRestartError(ctx, exit, err, retryCount)
}
