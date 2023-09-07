// This package contains the implementation of a simple websocket server with the following features:
//   - subscribe/unsubcribe to a publication
//   - publish heartbeats from server to client who subscribed the publciation on a regular basis
//   - echo with optional user provided request ID. User can also request an error to be returned
//   - interrupt/close client connections
package demowsserver

/*****************************************************************************/
/* BASE STRUCTURES                                                           */
/*****************************************************************************/

// Base struct for a message exchanged between the websocket server & client
type Message struct {
	// Mandatory message type indicator
	MsgType string `json:"type"`
}

// Structure that contains data of an error returned by the websocket server
type ErrorMessageData struct {
	// Error message
	Message string `json:"message"`
	// Optional error code
	Code string `json:"code,omitempty"`
	// User provided ID for its request
	ReqId string `json:"reqId,omitempty"`
}

// Base structure for a request message from client
type Request struct {
	Message
	// User provided ID for the request. Used to match with response in a async. Request-Response pattern
	ReqId string `json:"reqId,omitempty"`
}

// Base structure for a response message from server
type Response struct {
	Message
	// User provided ID for its request
	ReqId string `json:"reqId,omitempty"`
	// Status code -> OK or ERROR
	Status string `json:"status"`
	// Optional error -> must be there if status is ERROR
	Err *ErrorMessageData `json:"error,omitempty"`
	// Optional response data (can be absent even if status is OK)
	Data interface{} `json:"data,omitempty"`
}

// Struct for a error message when no other response is applicable
type ErrorMessage struct {
	Message
	ErrorMessageData
}

/*****************************************************************************/
/* SUBSCRIBE/UNSUBSCRIBE MESSAGES                                            */
/*****************************************************************************/

// Subscribe request message
type SubscribeRequest struct {
	Request
	// Topic to subscribe to
	Topic string `json:"topic"`
}

// Data of a SubscribeResponse
type SubscribeResponseData struct {
	// Topic client has subscribed to
	Topic string `json:"topic"`
}

// Subscribe response message
type SubscribeResponse struct {
	Response
	// Optional SubscribeResponse data
	Data *SubscribeResponseData `json:"data,omitempty"`
}

// Unsubscribe request message
type UnsubscribeRequest struct {
	Request
	// Topic to unsubscribe to
	Topic string `json:"topic"`
}

// Data of a UnsubscribeResponse
type UnsubscribeResponseData struct {
	// Topic client has unsubscribed to
	Topic string `json:"topic"`
}

// Unsubscribe response message
type UnsubscribeResponse struct {
	Response
	// Optional UnsubscribeResponse data
	Data *UnsubscribeResponseData `json:"data,omitempty"`
}

/*****************************************************************************/
/* ECHO MESSAGES                                                             */
/*****************************************************************************/

// Echo request message
type EchoRequest struct {
	Request
	// Message to be echoed by server
	Echo string `json:"echo"`
	// If true, the server will have to return an error instead of an echo
	Err bool `json:"err,omitempty"`
}

// Data of a EchoResponse message
type EchoResponseData struct {
	// Echoed message
	Echo string `json:"echo"`
}

// EchoResponse message
type EchoResponse struct {
	Response
	Data *EchoResponseData `json:"data,omitempty"`
}

/*****************************************************************************/
/* PUBLICATIONS                                                              */
/*****************************************************************************/

// Heartbeat publication
type Heartbeat struct {
	Message
	// Timestamp of the heartbeat publication
	Timestamp string `json:"timestamp"`
	// Connection uptime in seconds
	Uptime string `json:"uptime_seconds"`
}
