package wsclient

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Test mock fully implements interface it mocks
func TestMockInterfaceCompliance(t *testing.T) {
	var instance any = new(WebsocketClientMock)
	_, ok := instance.(WebsocketClientInterface)
	require.True(t, ok)
}
