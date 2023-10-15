package providers

import (
	"github.com/gbdevw/gowsclient/example/client/websocket"
	"github.com/gbdevw/gowsclient/wscengine/wsclient"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

func ProvideWebsocketClient(logger *zap.Logger, tracerProvider trace.TracerProvider) wsclient.WebsocketClientInterface {
	return websocket.NewExampleClientImpl(logger, tracerProvider)
}
