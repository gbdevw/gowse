package providers

import (
	"context"
	"net/http"

	"github.com/gbdevw/gowsclient/echowsserver"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

func ProvideEchoServer(lc fx.Lifecycle, logger *zap.Logger) *echowsserver.EchoWebsocketServer {
	// Build server - will listen on all interfaces on port 8081
	srv := echowsserver.NewEchoWebsocketServer(&http.Server{Addr: "0.0.0.0:8081"}, zap.NewStdLog(logger))
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
