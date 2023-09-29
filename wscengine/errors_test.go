package wscengine

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

/*************************************************************************************************/
/* TEST SUITES                                                                                   */
/*************************************************************************************************/

// Test suite used for EngineStartError unit tests
type EngineStartErrorUnitTestSuite struct {
	suite.Suite
}

// Run EngineStartErrorUnitTestSuite test suite
func TestEngineStartErrorUnitTestSuite(t *testing.T) {
	suite.Run(t, new(EngineStartErrorUnitTestSuite))
}

/*************************************************************************************************/
/* UNIT TESTS                                                                                    */
/*************************************************************************************************/

// Test Error
func (suite *WebsocketEngineOptionsUnitTestSuite) TestError() {
	// Expectations
	err := fmt.Errorf("root error")
	expected := fmt.Sprint("websocket engine failed to start: ", err)
	require.Equal(suite.T(), expected, EngineStartError{Err: err}.Error())
}

// Test Unwrap
func (suite *WebsocketEngineOptionsUnitTestSuite) TestUnwrap() {
	// Expectations
	expected := "inner error"
	require.Equal(suite.T(), expected, EngineStartError{
		Err: fmt.Errorf(expected),
	}.Unwrap().Error())
}
