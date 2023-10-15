package providers

import (
	"context"
	"net/url"
	"time"

	"github.com/gbdevw/gowsclient/example/client/configuration"
	"github.com/gbdevw/gowsclient/wscengine"
	"github.com/gbdevw/gowsclient/wscengine/wsadapters"
	"github.com/gbdevw/gowsclient/wscengine/wsclient"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/fx"
)

func ProvideWebsocketClienEngine(
	lc fx.Lifecycle,
	config configuration.Configuration,
	conn wsadapters.WebsocketConnectionAdapterInterface,
	client wsclient.WebsocketClientInterface,
	tracerProvider trace.TracerProvider) (*wscengine.WebsocketEngine, error) {
	// Parse target server url
	u, err := url.Parse(config.ServerUrl)
	if err != nil {
		return nil, err
	}
	// Build engine
	engine, err := wscengine.NewWebsocketEngine(u, conn, client, nil, tracerProvider)
	if err != nil {
		return nil, err
	}
	// Register Start and Stop hooks to Start and Stop the engine
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			// Attempt 3 times to start
			attempts := 0
			var err error = nil
			for attempts < 3 {
				err := engine.Start(ctx)
				if err == nil {
					// Exit - success
					return nil
				}
				// Wait a bit before retry
				time.Sleep(2 * time.Second)
			}
			// Return error
			return err
		},
		OnStop: func(ctx context.Context) error {
			return engine.Stop(ctx)
		},
	})
	// Return engine
	return engine, nil
}
