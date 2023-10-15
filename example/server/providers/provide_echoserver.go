package providers

import (
	"context"

	"github.com/gbdevw/gowsclient/echowsserver"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

func ProvideEchoServer(lc fx.Lifecycle, logger *zap.Logger) *echowsserver.EchoWebsocketServer {
	// Build server - will listen on localhost:8080
	srv := echowsserver.NewEchoWebsocketServer(nil, zap.NewStdLog(logger))
	// Register Start and Stop hooks to Start and Stop the server
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			// Start server
			return srv.Start()
		},
		OnStop: func(ctx context.Context) error {
			return srv.Stop()
		},
	})
	// Return the server
	return srv
}
