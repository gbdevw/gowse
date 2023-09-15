package configuration

import "os"

type Configuration struct {
	// URL of the target websocket server
	ServerUrl string
	// Indicates whether tracing is enabled or not
	TracingEnabled string
	// UURL to the Jaeger tracing backend
	JaegerTracingBackendEndpoint string
}

func LoadConfiguration() Configuration {
	return Configuration{
		ServerUrl:                    os.Getenv("WSCEX_SERVER_URL"),
		TracingEnabled:               os.Getenv("WSCEX_TRACING_ENABLED"),
		JaegerTracingBackendEndpoint: os.Getenv("WSCEX_TRACING_JAEGER_ENDPOINT"),
	}
}
