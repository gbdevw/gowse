package providers

import (
	"github.com/gbdevw/gowsclient/example/client"
	"github.com/gbdevw/gowsclient/pkg/wscengine/wsclient"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

func ProvideWebsocketClient(logger *zap.Logger, tracerProvider trace.TracerProvider) wsclient.WebsocketClientInterface {
	return client.NewExampleClientImpl(logger, tracerProvider)
}
