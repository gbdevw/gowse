// This package contains the implementation of a simple websocket server with the following features:
//   - subscribe/unsubcribe to a publication
//   - publish heartbeats from server to client who subscribed the publciation on a regular basis
//   - echo with optional user provided request ID. User can also request an error to be returned
//   - interrupt/close client connections
package demowsserver

// Constants used in messages
const (
	// Message type: error
	MSG_TYPE_ERROR = "error"
	// Message type: subscribe - request
	MSG_TYPE_SUBSCRIBE_REQUEST = "subscribe_request"
	// Message type: subscribe - response
	MSG_TYPE_SUBSCRIBE_RESPONSE = "subscribe_response"
	// Message type: unsubscribe - request
	MSG_TYPE_UNSUBSCRIBE_REQUEST = "unsubscribe_request"
	// Message type: unsubscribe - response
	MSG_TYPE_UNSUBSCRIBE_RESPONSE = "unsubscribe_response"
	// Message type: echo - request
	MSG_TYPE_ECHO_REQUEST = "echo_request"
	// Message type: echo - response
	MSG_TYPE_ECHO_RESPONSE = "echo_response"
	// Message type: heartbeat
	MSG_TYPE_HEARTBEAT = "heartbeat"
	// Topic for heartbeats
	TOPIC_HEARTBEAT = "heartbeat"
	// Response status OK
	RESPONSE_STATUS_OK = "OK"
	// Response status for an error
	RESPONSE_STATUS_ERROR = "ERROR"
	// Error code used when client has already subscribed to a topic
	ERROR_ALREADY_SUBSCRIBED = "ALREADY_SUBSCRIBED"
	// Error code used when client has not subscribed to a topic and asks to unsubscribe
	ERROR_NOT_SUBSCRIBED = "NOT_SUBSCRIBED"
	// Error code used when client ask echo to return an error
	ERROR_ECHO_ASKED_BY_CLIENT = "ASKED_BY_CLIENT"
	// Error code used when topic asked by client is not known
	ERROR_TOPIC_NOT_FOUND = "TOPIC_NOT_FOUND"
	// Error returned when a message could not be handled by the server (either unkown message type or message type is not there)
	ERROR_UNKNOWN_MESSAGE_TYPE = "UNKNOWN_MESSAGE_TYPE"
	// Error returned when a message could not be unmarshalled or is malformatted
	ERROR_BAD_REQUEST = "BAD_REQUEST"
)

// Constants used for tracing purpose
const (
	demowsserver_instrumentation_id                         = "DemoWebsocketServer"
	demowsserver_span_start                                 = demowsserver_instrumentation_id + ".Start"
	demowsserver_span_attr_start_host                       = "host"
	demowsserver_span_stop                                  = demowsserver_instrumentation_id + ".Stop"
	demowsserver_span_terminate                             = demowsserver_instrumentation_id + ".Terminate"
	demowsserver_span_accept                                = demowsserver_instrumentation_id + ".Accept"
	demowsserver_span_close_client_connections              = demowsserver_instrumentation_id + ".CloseClientConnections"
	demowsserver_event_closed_client_connections            = "ClosedClientConnection"
	demowsserver_event_attr_closed_client_connection_id     = "id"
	demowsserver_span_cleanup                               = demowsserver_instrumentation_id + ".Cleanup"
	demowsserver_event_cleanup                              = "Cleanup"
	demowsserver_event_cleanup_count_attr                   = "count"
	demowsserver_event_cleanup_exit                         = "Exit"
	demowsserver_event_cleanup_exit_loops_attr              = "loopsAfterShutdown"
	demowsserver_span_client_session                        = demowsserver_instrumentation_id + ".ClientSession"
	demowsserver_span_client_session_run                    = demowsserver_span_client_session + ".Run"
	demowsserver_span_client_session_run_id_attr            = "id"
	demowsserver_event_client_session_exit                  = "Exit"
	demowsserver_event_client_session_exit_reason_code_attr = "code"
	demowsserver_event_client_session_exit_reason_attr      = "reason"
	demowsserver_span_client_session_run_handle             = demowsserver_span_client_session_run + ".Handle"
	demowsserver_span_attr_client_session_id                = "sessionId"
	demowsserver_metric_connections_counter                 = "connections_total"
	demowsserver_metric_active_connections_gauge            = "connections_active"
	demowsserver_metric_start_unix_gauge                    = "start_unix_timestamp_seconds_info"
	demowsserver_metric_started_gauge                       = "started_info"
	demowsserver_span_echo                                  = demowsserver_span_client_session_run_handle + ".Echo"
	demowsserver_span_attr_echo_return_error                = "returnError"
	demowsserver_span_attr_echo_req_id                      = "reqId"
	demowsserver_span_subscribe                             = demowsserver_span_client_session_run_handle + ".Subscribe"
	demowsserver_span_attr_subscribe_topic                  = "topic"
	demowsserver_span_attr_subscribe_req_id                 = "reqId"
	demowsserver_span_unsubscribe                           = demowsserver_span_client_session_run_handle + ".Unsubscribe"
	demowsserver_span_attr_unsubscribe_topic                = "topic"
	demowsserver_span_attr_unsubscribe_req_id               = "reqId"
	demowsserver_span_heartbeat                             = demowsserver_span_client_session + ".Heartbeat"
	demowsserver_span_attr_heartbeat_client_id              = "id"
	demowsserver_span_heartbeat_unsubscribe                 = demowsserver_span_heartbeat + ".Unsubscribe"
	demowsserver_span_write_error_message                   = demowsserver_span_client_session + ".WriteErrorMessage"
	demowsserver_span_attr_error_message_code               = "code"
	demowsserver_span_attr_error_message                    = "message"
	demowsserver_span_write_echo_response                   = demowsserver_span_client_session + ".WriteEchoResponseMessage"
	demowsserver_span_attr_write_echo_response_status       = "status"
	demowsserver_span_attr_write_echo_response_req_id       = "reqId"
	demowsserver_span_write_subscribe_response              = demowsserver_instrumentation_id + ".WriteSubscribeResponseMessage"
	demowsserver_span_write_subscribe_response_status       = "status"
	demowsserver_span_write_subscribe_response_req_id       = "reqId"
	demowsserver_span_write_unsubscribe_response            = demowsserver_instrumentation_id + ".WriteUnsubscribeResponseMessage"
	demowsserver_span_write_unsubscribe_response_status     = "status"
	demowsserver_span_write_unsubscribe_response_req_id     = "reqId"
)

// Other constants used internally
const (
	// Number of seconds ellapsed between closed connection cleanup
	srv_cleanup_frequency_seconds = 5
	// Max. number of connection cleanup performed on shutdown before exiting.
	// When shutting down, the server will wait all connections to be closed before finishing Stop function.
	srv_cleanup_max_loop_after_shutdown = 2
	// Timeout (seconds) to perform complete server shutdown.
	// Will be (number of cleanup loops + 2) * cleanup loop frequency to ensure enough time is provided.
	srv_shutdown_timeout = (srv_cleanup_max_loop_after_shutdown + 2) * srv_cleanup_frequency_seconds
)
