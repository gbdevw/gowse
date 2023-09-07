# Callbacks explained

## OnOpen

The logic contained in OnOpen callback is executed each time the websocket container (re)connects to the server. 

During the OnOpen callback, user can manually and synchronously send/receive messages without the engine getting in the way. This is useful in the case you are required to send some specific messages after connecting to the server. Once OnOpen callback is executed, the engine will continuously read incoming messages and use appropriate callbacks (OnMessage, OnReadError, OnClose).

User can return an error from OnOpen callback if something goes wrong: 
- If the engine is restarting, it will automatically restart again unless provided exit function has been called by user code. 
- If the engine is starting, during Start method call,, it will exit and Start method will return the error from OnOpen.

In case user wants to stop the engine from OnOpen calllback, user can call the provided exit function to stop the engine.

## OnMessage

This callback is called each time a message is received from the server.