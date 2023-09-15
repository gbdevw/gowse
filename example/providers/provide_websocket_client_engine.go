package providers

import (
	"context"
	"net/url"

	"github.com/gbdevw/gowsclient/example/configuration"
	"github.com/gbdevw/gowsclient/pkg/wscengine"
	"github.com/gbdevw/gowsclient/pkg/wscengine/wsadapters"
	"github.com/gbdevw/gowsclient/pkg/wscengine/wsclient"
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
			return engine.Start(ctx)
		},
		OnStop: func(ctx context.Context) error {
			return engine.Stop(ctx)
		},
	})
	// Return engine
	return engine, nil
}
