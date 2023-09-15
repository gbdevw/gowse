# Callbacks explained

This chapter contains further explanation of each callback that needs to be implemented by the websocket client used by the engine.

## OnOpen

The logic contained in the OnOpen callback is executed each time the websocket container (re)connects to the server. 

During the OnOpen callback, the user can manually send/receive messages synchronously without the engine getting in the way. This is useful if you need to send some specific messages after connecting to the server. Once the OnOpen callback is executed, the engine will continuously read incoming messages and use the appropriate callbacks (OnMessage, OnReadError, OnClose).

The user can return an error from the OnOpen callback if something goes wrong: 
- If the engine is restarting, it will automatically restart again unless the exit function provided has been called by user code. 
- If the engine is started during the Start method call, it will exit and the Start method will return the error from OnOpen.

If the user wants to stop the engine from the OnOpen callback, the user can call the provided exit function to stop the engine.

## OnMessage

This callback is called each time a message is received from the server.

While the message is being processed, other engine goroutines, if any, since the engine can be configured to have only one goroutine, will process incoming messages in parallel.

**Engine behavior on exit/restart call:**

- No other messages will be read if restart or exit is called.
- Engine will stop after OnMessage is completed: OnClose callback is called and then the connection is closed. Depending on which function was called, the engine will restart or stop for good.
- All pending messages will be discarded. The user can continue to read and send messages in this callback and/or in the OnClose callback until conditions are met to stop the engine and close the websocket connection.

## OnReadError

This callback is called each time an error is received when reading messages from the websocket server that is not caused by the connection being closed.

The callback is called by the engine goroutine that encountered the error. All engine goroutines will block until the callback is completed. This prevents other messages and events from being processed by the engine while the error is being handled.

The engine will restart after OnReadError has finished if one of the following conditions is met
- The websocket connection is closed and the Exit function has not been called.
- The restart function has been called.

Otherwise, the engine will either continue to process messages on the same connection or shut down if the exit function has been called.

Do not close the websocket connection manually: It will be automatically closed if necessary after the OnClose callback has been completed.

**Engine behaviour on Exit/Restart call:**

- No other messages are read when restart or exit is called.
- Engine will stop after OnReadError: OnClose callback is called and then the connection is closed. Depending on which function was called, the engine will restart or stop for good.
- All pending messages will be discarded. The user can continue to read and send messages in this callback and/or in the OnClose callback until conditions are met to stop the engine and close the websocket connection.

## OnClose

Callback is called when the websocket connection is closed or about to be closed after a Stop method call or a call to the provided restart/exit functions. Callback is called once by the engine: the engine will not exit or restart until the callback has been completed.

Callback can return an optional CloseMessageDetails which will be used to build the close message sent to the server if the connection needs to be closed after OnClose has finished. In such a case, if the returned value is nil, the engine will use 1001 "Going Away" as the close message.

Do not close the websocket connection here if it is still open: It will be automatically closed by the engine with a close message.

## OnCloseError

Callback called if an error occurred when the engine called the conn.Close method during the shutdown phase.

## OnRestartError

Callback called in case an error or a timeout occured when engine tried to restart.