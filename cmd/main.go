package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"time"

	demowsserver "gitlab.com/lake42/go-websocket-client/pkg/demowsserver"
)

func main() {
	// Create and start websocket server -> localhost:8080
	srv, err := demowsserver.NewDemoWebsocketServer(context.Background(), nil, nil, nil)
	if err != nil {
		panic(err)
	}
	srv.Start()
	// Wait for shutdown
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt)
	<-sigs
	log.Println("Application shutdown initiated")
	// Close server
	srv.Stop()
	time.Sleep(5 * time.Second)
}
