# Go Websocket Client

A callback based framework and engine to build websocket clients in Golang, inspired from javax.websocket.

## Features

- **Implementing a websocket client in Go with six callbacks**: OnOpen, OnMessage, OnClose, OnReadError, OnCloseError & OnRestartError.
- **Plug in the websocket library of your choice**: Adapters for nhooyr/websocket and (COMING SOON) gorilla/websocket are provided.
- **Batteries included**: Included websocket client engine manages the websocket connection, auto-reconnects when necessary and calls appropriate user-provided callbacks.
- **Predictable concurrency model and lifecycle**: 
    - Engine allows users to have 1 to N routines reading and processing messages simultaneously.
    - Engine provides a 'pause' button that users can use in case they want to temporarily 'pilot' the websocket connection.
    - [The way the engine works, handles errors, shutdown etc... is documented here](./documentation/websocket_client_engine_operations.md)
- **Observability**: All components are already instrumented using Opentelemetry. Users can either plug in their own TracerProvider or let libraries fallback to the global TracerProvider. User provided callbacks receive all tracing data through their context parameter. This allows users to fully trace message processing from the server back to their code.

## Usage tips

### OnOpen callback

The logic contained in the OnOpen callback is executed each time the websocket container (re)connects to the server. During the OnOpen callback, the user can manually send/receive messages synchronously without the engine getting in the way. This is useful if you need to send some specific messages after connecting to the server. Once the OnOpen callback is executed, the engine will continuously read incoming messages and use the appropriate callbacks.

### Request-Response over Websocket

The request-response pattern can be tricky for a websocket application client, because a response to a request can be mixed with other publications/responses. In addition, you have to deal with a callback-based engine that executes your client application logic. There are two ways to deal with the request-response pattern:

    1. The websocket application supports custom IDs for requests and responses (good practice). In this case, your websocket client can implement its own logic to
        - Add custom IDs to requests (messages sent to the server via websocket)
        - Send the request
        - Store the request data and its ID in a map
        - In the OnMessage callback, match responses and requests using the provided ID and execute further logic.
    
    2. The websocket application does not support custom IDs for requests and responses. In this case, the websocket container provides a mutex that can be locked to 'pause' the engine until you issue your request and manually process incoming messages. When you are finished with your request (and response), you can unlock the mutex to restart the engine.

### Close the websocket connection

Use the websocket container's Stop() method to gracefully close the websocket connection and call the appropriate callback (OnClose with the ClientInitiated flag). It is not recommended to close the websocket connection manually: if the auto-reconnect feature is enabled, you will just get an OnClose call and the websocket connection will be reopened.

## Licence

Apache License Version 2.0, January 2004