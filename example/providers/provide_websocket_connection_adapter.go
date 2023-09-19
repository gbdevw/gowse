package providers

import (
	"net/http"

	"github.com/gbdevw/gowsclient/wscengine/wsadapters"
	wsadapternhooyr "github.com/gbdevw/gowsclient/wscengine/wsadapters/nhooyr"
	"go.opentelemetry.io/otel/trace"
	"nhooyr.io/websocket"
)

func ProviderWebsocketConnectionAdapter(tracerProvider trace.TracerProvider) wsadapters.WebsocketConnectionAdapterInterface {
	// Return a websocket connection adapter which uuses nhooyr websocket library under the hood
	return wsadapternhooyr.NewNhooyrWebsocketConnectionAdapter(&websocket.DialOptions{
		HTTPClient: &http.Client{
			Timeout: 0,
		},
	}, tracerProvider)
}
