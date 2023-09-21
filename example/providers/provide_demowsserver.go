package providers

import (
	"context"

	"github.com/gbdevw/gowsclient/demowsserver"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/fx"
)

// Provide the websocket server and register start/stop hooks to start/stop the server
func ProvideWebsocketServer(lc fx.Lifecycle, ctx context.Context, tracerProvider trace.TracerProvider) (*demowsserver.DemoWebsocketServer, error) {
	// Build server
	srv, err := demowsserver.NewDemoWebsocketServer(ctx, nil, tracerProvider, nil)
	if err != nil {
		return nil, err
	}
	// Register Start and Stop hooks to Start and Stop the websocket server
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return srv.Start()
		},
		OnStop: func(ctx context.Context) error {
			return srv.Stop()
		},
	})
	// Return the server
	return srv, nil
}
