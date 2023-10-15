package providers

import (
	"github.com/gbdevw/gowsclient/wscengine/wsadapters"
	"github.com/gbdevw/gowsclient/wscengine/wsadapters/gorilla"
	"go.opentelemetry.io/otel/trace"
)

func ProviderWebsocketConnectionAdapter(tracerProvider trace.TracerProvider) wsadapters.WebsocketConnectionAdapterInterface {
	// Return a websocket connection adapter which uuses nhooyr websocket library under the hood
	return gorilla.NewGorillaWebsocketConnectionAdapter(nil, nil)
}
