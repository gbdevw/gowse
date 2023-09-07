# Go Websocket Client

A callback based framework and engine to build websocket clients in Golang, inspired from javax.websocket.

## Features

- Implement a websocket client in Go with six callbacks: OnOpen, OnMessage, OnClose, OnReadError, OnCloseError & OnRestartError
- Plug the websocket library of your choice: Adapters are provided for nhooyr/websocket and (UPCOmING) gorilla/websocket
- Batteries included: Provided websocket client engine manages the websocket connection, auto-reconnects if needed and calls appriopriate user provided callbacks.
- Predictable concurrency model and lifecycle: 
    - Engine allows users to have 1 to N goroutines which concurrently read and process messages.
    - Engine provides a 'pause' button users can use in case they want to temporarely 'pilot' the websocket connection.
    - [The way engine operates, handles errors, shutdown etc... is documented here](./documentation/websocket_client_engine_operations.md)
- Observability: All components are already instrumented using Opentelemetry. Users can either plug their own TracerProvider or let librairies fallback on the global TracerProvider. User provided callbacks receive all tracing data through their context parameter. Therefore, users can fully trace message processing from the server up to their code.

## Usage tips

### OnOpen callback

The logic contained in OnOpen callback is executed each time the websocket container (re)connects to the server. During the OnOpen callback, the user can manually and synchronously send/receive messages without the engine getting in the way. This is useful in the case you are required to send some specific messages after connecting to the server. Once OnOpen callback is executed, the engine will continuously read incoming messages and use appropriate callbacks (OnMessage, OnError, OnClose).

### Request-Response over Websocket

Request-Response pattern can be tricky for a websocket application client because a response to a request can be mixed with other publications/responses. On top of that, you must deal with a callback based engine that executes your client application logic. Here are two ways to deal with request-response pattern:

    1. The websocket application supports user defined IDs for requests and responses (good practise). In that case, your websocket client can implement its own logic to:
        - Add user defined IDs to requests (messages sent to server over websocket)
        - Send the request
        - Store the request data and its ID in a map
        - In OnMessage callback, reconcialiate responses and requests by using the provided ID and execute further logic.
    
    2. The websocket application does not suppport user defined IDs for requests and responses. In that case, the websocket container provides a mutex that can be locked to 'pause' the engine by the time you issue your request and manually process incoming messages. Once your are done with your request (and response), you can unlock the mutex to resume the engine.

### Close the websocket connection

Use the Stop() method of the websocket container to gracefully close the websocket connection and call appropriate calllback (OnClose with ClientInitiated flag). It is not recommended to manually close the websocket connection: if the auto-reconnect feature is enabled, you will just get a OnClose call and websocket connection will be reopened.

## Licence

Apache License Version 2.0, January 2004