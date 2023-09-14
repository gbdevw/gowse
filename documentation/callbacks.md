# Callbacks explained

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