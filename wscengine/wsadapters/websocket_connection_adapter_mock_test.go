package wsadapters

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Test mock fully implements interface it mocks
func TestMockInterfaceCompliance(t *testing.T) {
	var instance any = new(WebsocketConnectionAdapterInterfaceMock)
	_, ok := instance.(WebsocketConnectionAdapterInterface)
	require.True(t, ok)
}
