package example

import (
	"github.com/gbdevw/gowsclient/example/configuration"
	"github.com/gbdevw/gowsclient/example/providers"
	"go.uber.org/fx"
)

func main() {
	fx.New(
		fx.Provide(providers.ProvideApplicationContext),
		fx.Provide(configuration.LoadConfiguration),
		fx.Provide(providers.ProvideLogger),
		fx.Provide(providers.ProvideTracerProvider),
		fx.Provide(providers.ProviderWebsocketConnectionAdapter),
		fx.Provide(providers.ProvideWebsocketClient),
		// Use invoke to force dependency to be isntanciated and hooks to be registered and execuuted
		fx.Invoke(providers.ProvideWebsocketClienEngine),
	).Run()
}
