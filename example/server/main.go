package main

import (
	"github.com/gbdevw/gowsclient/example/server/providers"
	"go.uber.org/fx"
)

func main() {
	fx.New(
		fx.Provide(providers.ProvideLogger),
		fx.Provide(providers.ProvideEchoServer),
		// Use invoke to force dependency to be instanciated and hooks to be registered and executed
		fx.Invoke(providers.ProvideEchoServer),
	).Run()
}
