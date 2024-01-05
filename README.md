# GO WebSocket Engine
![Coverage](https://img.shields.io/badge/Coverage-95.1%25-brightgreen)

A callback based framework and engine to build websocket clients in Golang, inspired from javax.websocket.

## Status

- This is an alpha release: The websocket engine can be used in other projects, but the API and code stability are not yet guaranteed, nor is the complete absence of bugs.
- A stable release will be released once the project has been successfully used in a first internal project.

## Contribute

- This project is actively maintained. 
- Please file an issue if you want to report bugs or make recommendations about the code or the way it is managed.

## Features

- **Implementing a websocket client in Go with six callbacks**: OnOpen, OnMessage, OnClose, OnReadError, OnCloseError & OnRestartError.
- **Plug in the websocket library of your choice**: Adapters for nhooyr/websocket and (COMING SOON) gorilla/websocket are provided.
- **Batteries included**: Included websocket client engine manages the websocket connection, auto-reconnects when necessary and calls appropriate user-provided callbacks.
- **Predictable concurrency model and lifecycle**: 
    - Engine allows users to have 1 to N routines reading and processing messages simultaneously.
    - Engine provides a 'pause' button that users can use in case they want to temporarily 'pilot' the websocket connection.
    - [The way the engine works, handles errors, shutdown etc... is documented here](./documentation/websocket_client_engine_operations.md)
- **Observability**: All components are already instrumented using Opentelemetry. Users can either plug in their own TracerProvider or let libraries fallback to the global TracerProvider. User provided callbacks receive all tracing data through their context parameter. This allows users to fully trace message processing from the server back to their code.

## Usage

Import the websocket engine in your go project:

```
go get -u github.com/gbdevw/gowse/wscengine
```

Pick the websocket framework adapter of your choice:

```
go get -u github.com/gbdevw/gowse/wscengine/wsadapters/gorilla
go get -u github.com/gbdevw/gowse/wscengine/wsadapters/nhoyr
```

### Callbacks

Additional explanation of callbacks that need to be implemented by the websocket client endpoint can be found [here](./documentation/callbacks.md).

### Engine lifecycle

Additional explanation about how the engine works and interact with users provided callbacks can be found [here](./documentation/websocket_client_engine_operations.md)

### Example

An example of a client for the [echowsserver](./echowsserver/echo_websocket_server.go) is available under [example/client](./example/client/websocket/client.go). The client opens a connection to the echo websocket server and exchange messages with the server every 5 seconds. 

When the connection is established, the client engine calls OnOpen callback, which sends an initial message to the server. The server responds with an echo. When an echo is received, the OnMessage callback is called by the client engine. The callback implementation is very simple: It waits 5 seconds before sending a new message to the server, which replies. The response triggers the OnMessage callback again. This continues until either the server or the client is shut down.

The simplest way to run the example is to use the [docker compose file](./compose.yaml):

```
docker-compose up
```

The docker compose file will start 3 containers:
- The websocket server
- The client
- A all-in-one jaeger instance

The user can connect to the the jaeger instance on port 16686 to see the traces produced by the client (and the websocket engine).

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