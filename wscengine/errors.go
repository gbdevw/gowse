package wscengine

import "fmt"

/*************************************************************************************************/
/* ENGINE START ERROR                                                                            */
/*************************************************************************************************/

// Specific error type for errors which occurs when engine starts.
type EngineStartError struct {
	// Embedded error
	Err error
}

func (err EngineStartError) Error() string {
	return fmt.Sprintf("websocket engine failed to start: %v", err.Err)
}

func (err EngineStartError) Unwrap() error {
	return err.Err
}
