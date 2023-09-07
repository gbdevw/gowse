package wsclientengine

/*************************************************************************************************/
/* ENGINE START ERROR                                                                            */
/*************************************************************************************************/

// Specific error type for errors which occurs when engine starts.
type EngineStartError struct {
	// Embedded error
	Err error
}

func (err EngineStartError) Error() string {
	return "websocket engine failed to start"
}

func (err EngineStartError) Unwrap() error {
	return err.Err
}
