// The package defines an interface to adapt 3rd parties websocket libraries to websocket engine.
package wsadapters

import (
	"context"
	"net/http"
	"net/url"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// A decorator which can be used to automatically instrument implementations of
// WebsocketConnectionAdapterInterface.
type WebsocketConnectionAdapterInstrumentationDecorator struct {
	// Decorated WebsocketConnectionAdapterInterface implementation
	decorated WebsocketConnectionAdapterInterface
	// Tracer used for instrumentation
	tracer trace.Tracer
}

// # Description
//
// Create a new decorator which will atomatically instrument the provided implementation of
// WebsocketConnectionAdapterInterface.
func NewWebsocketConnectionAdapterInstrumentationDecorator(
	decorated WebsocketConnectionAdapterInterface,
	tracerProvider trace.TracerProvider,
) (*WebsocketConnectionAdapterInstrumentationDecorator, error) {
	// If tracerProvider is nil
	if tracerProvider == nil {
		tracerProvider = otel.GetTracerProvider()
	}
	// Bild and return decorator
	return &WebsocketConnectionAdapterInstrumentationDecorator{
		decorated: decorated,
		tracer:    tracerProvider.Tracer(pkgName, trace.WithInstrumentationVersion(pkgVersion)),
	}, nil
}

// Decorate and instrument the Dial method of a WebsocketConnectionAdapterInterface implementation.
func (decorator *WebsocketConnectionAdapterInstrumentationDecorator) Dial(ctx context.Context, target url.URL) (*http.Response, error) {
	// Start span
	ctx, span := decorator.tracer.Start(ctx, spanDial,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String(attrUrl, target.String()),
		))
	defer span.End()
	// Call decorated Dial method
	resp, err := decorator.decorated.Dial(ctx, target)
	if err != nil {
		// Trace error
		span.RecordError(err)
		span.SetStatus(codes.Error, codes.Error.String())
	}
	// Return results
	return resp, err
}

// Decorate and instrument the Close method of a WebsocketConnectionAdapterInterface implementation.
func (decorator *WebsocketConnectionAdapterInstrumentationDecorator) Close(ctx context.Context, code StatusCode, reason string) error {
	// Start span
	ctx, span := decorator.tracer.Start(ctx, spanClose,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.Int(attrCloseCode, int(code)),
			attribute.String(attrCloseReason, reason),
		))
	defer span.End()
	// Call decorated Close method
	err := decorator.decorated.Close(ctx, code, reason)
	if err != nil {
		// Trace error
		span.RecordError(err)
		span.SetStatus(codes.Error, codes.Error.String())
	}
	// Return decorated results
	return err
}

// Decorate and instrument the Close method of a WebsocketConnectionAdapterInterface implementation.
func (decorator *WebsocketConnectionAdapterInstrumentationDecorator) Ping(ctx context.Context) error {
	// Start span
	ctx, span := decorator.tracer.Start(ctx, spanPing, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()
	// Call decorated Ping method
	err := decorator.decorated.Ping(ctx)
	if err != nil {
		// Trace error
		span.RecordError(err)
		span.SetStatus(codes.Error, codes.Error.String())
	}
	// Return decorated results
	return err
}

// Decorate and instrument the Read method of a WebsocketConnectionAdapterInterface implementation.
func (decorator *WebsocketConnectionAdapterInstrumentationDecorator) Read(ctx context.Context) (MessageType, []byte, error) {
	// Start span
	ctx, span := decorator.tracer.Start(ctx, spanRead, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()
	// Call decorated Read method
	msgType, msg, err := decorator.decorated.Read(ctx)
	if err != nil {
		// Trace error
		span.RecordError(err)
		span.SetStatus(codes.Error, codes.Error.String())
	}
	// Add received event
	span.AddEvent(eventReceived, trace.WithAttributes(
		attribute.Int(attrMessageByteSize, len(msg)),
		attribute.Int(attrMessageType, int(msgType)),
	))
	// Return decorated results
	return msgType, msg, err
}

// Decorate and instrument the Read method of a WebsocketConnectionAdapterInterface implementation.
func (decorator *WebsocketConnectionAdapterInstrumentationDecorator) Write(ctx context.Context, msgType MessageType, msg []byte) error {
	// Start span
	ctx, span := decorator.tracer.Start(ctx, spanWrite,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.Int(attrMessageByteSize, len(msg)),
			attribute.Int(attrMessageType, int(msgType)),
		))
	defer span.End()
	// Call decorated Write method
	err := decorator.decorated.Write(ctx, msgType, msg)
	if err != nil {
		// Trace error
		span.RecordError(err)
		span.SetStatus(codes.Error, codes.Error.String())
	}
	// Return decorated results
	return err
}

// Simple proxy for non-instrumented getter
func (decorator *WebsocketConnectionAdapterInstrumentationDecorator) GetUnderlyingWebsocketConnection() any {
	return decorator.decorated.GetUnderlyingWebsocketConnection()
}
