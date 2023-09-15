package wsadapters

import "fmt"

/*************************************************************************************************/
/* WEBSOCKET CLOSE ERROR                                                                         */
/*************************************************************************************************/

// Error used by websocket connection to signal connection has been closed
type WebsocketCloseError struct {
	// Status code used or received when connection has been closed. If websocket connection has
	// been closed and no close message has been received, 1006 should be used.
	//
	// https://www.rfc-editor.org/rfc/rfc6455.html#section-7.1.5
	Code StatusCode
	// Optional close reason used/received when connection has been closed.
	//
	// https://www.rfc-editor.org/rfc/rfc6455.html#section-7.1.6
	Reason string
	// Embedded error if any. Can be the error returned by the underlying websocket library by Read
	// method when websocket is closed.
	Err error
}

func (err WebsocketCloseError) Error() string {
	return fmt.Sprintf("connection has been closed: %d - %s", err.Code, err.Reason)
}

func (err WebsocketCloseError) Unwrap() error {
	return err.Err
}
