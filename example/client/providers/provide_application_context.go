package providers

import "context"

func ProvideApplicationContext() context.Context {
	return context.Background()
}
