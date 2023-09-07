// Package which contains a WebsocketConnectionAdapterInterface implementation for
// nhooyr/websocket library (https://github.com/nhooyr/websocket).
package wsadapternhooyr

/*************************************************************************************************/
/* TRACING RELATED CONSTANTS                                                                     */
/*************************************************************************************************/

const (
	// Package name used by tracer
	packageName = "wsclientadpternhooyr"
	// Package version used by tracer
	packageVersion = "0.0.0"

	root                  = "Websocket.Client.Adapter.Nhooyr"
	dialSpan              = root + ".Dial"
	dialSpanUrlAttr       = "url"
	dialCloseSpan         = dialSpan + ".Close"
	dialCloseSpanCodeAttr = "code"
	closeSpan             = root + ".Close"
	closeSpanCodeAttr     = "code"
	pingSpan              = root + ".Ping"
	writeSpan             = root + ".Write"
	writeSpanBytesAttr    = "bytes"
	writeSpanTypeAttr     = "type"
	readSpan              = root + ".Read"
	rcvdEvent             = "MessageRead"
	rcvdEventBytesAttr    = "bytes"
	rcvdEventTypeAttr     = "type"
)
