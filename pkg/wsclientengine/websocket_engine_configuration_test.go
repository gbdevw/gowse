package wsclientengine

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

/*************************************************************************************************/
/* TEST SUITES                                                                                   */
/*************************************************************************************************/

// Test suite used for WebsocketEngineOptions unit tests
type WebsocketEngineOptionsUnitTestSuite struct {
	suite.Suite
}

// Run WebsocketEngineOptionsUnitTestSuite test suite
func TestWebsocketEngineOptionsUnitTestSuite(t *testing.T) {
	suite.Run(t, new(WebsocketEngineOptionsUnitTestSuite))
}

/*************************************************************************************************/
/* UNIT TESTS                                                                                    */
/*************************************************************************************************/

// Test methods used to set options.
func (suite *WebsocketEngineOptionsUnitTestSuite) TestSetters() {
	// Expectations
	expectedAutoReconnect := false
	expectedAutoReconnectRetryDelayBaseSeconds := 5
	expectedAutoReconnectRetryDelayMaxExponent := 3
	expectedOnOpenTimeoutMs := int64(1)
	expectedReaderRoutinesCount := 5
	expectedStopTimeoutMs := int64(1)
	// Create options with default settings and set options
	opts := NewWebsocketEngineConfigurationOptions().
		WithAutoReconnect(expectedAutoReconnect).
		WithAutoReconnectRetryDelayBaseSeconds(expectedAutoReconnectRetryDelayBaseSeconds).
		WithAutoReconnectRetryDelayMaxExponent(expectedAutoReconnectRetryDelayMaxExponent).
		WithOnOpenTimeoutMs(expectedOnOpenTimeoutMs).
		WithReaderRoutinesCount(expectedReaderRoutinesCount).
		WithStopTimeoutMs(expectedStopTimeoutMs)
	// Assertions
	require.Equal(suite.T(), expectedAutoReconnect, opts.AutoReconnect)
	require.Equal(suite.T(), expectedAutoReconnectRetryDelayBaseSeconds,
		opts.AutoReconnectRetryDelayBaseSeconds)
	require.Equal(suite.T(), expectedAutoReconnectRetryDelayMaxExponent,
		opts.AutoReconnectRetryDelayMaxExponent)
	require.Equal(suite.T(), expectedOnOpenTimeoutMs, opts.OnOpenTimeoutMs)
	require.Equal(suite.T(), expectedReaderRoutinesCount, opts.ReaderRoutinesCount)
	require.Equal(suite.T(), expectedStopTimeoutMs, opts.StopTimeoutMs)
}

// Test option validation
func (suite *WebsocketEngineOptionsUnitTestSuite) TestValidate() {
	// Validate default options are valid
	err := Validate(NewWebsocketEngineConfigurationOptions())
	require.NoError(suite.T(), err)
	// Test invalid AutoReconnectRetryDelayBaseSeconds
	err = Validate(NewWebsocketEngineConfigurationOptions().
		WithAutoReconnectRetryDelayBaseSeconds(0))
	require.Error(suite.T(), err)
	// Test invalid AutoReconnectRetryDelayMaxExponent
	err = Validate(NewWebsocketEngineConfigurationOptions().
		WithAutoReconnectRetryDelayMaxExponent(0))
	require.Error(suite.T(), err)
	// Test invalid OnOpenTimeoutMs
	err = Validate(NewWebsocketEngineConfigurationOptions().
		WithOnOpenTimeoutMs(-1))
	require.Error(suite.T(), err)
	// Test invalid ReaderRoutinesCount
	err = Validate(NewWebsocketEngineConfigurationOptions().
		WithReaderRoutinesCount(-1))
	require.Error(suite.T(), err)
	// Test invalid StopTimeoutMs
	err = Validate(NewWebsocketEngineConfigurationOptions().
		WithStopTimeoutMs(-1))
	require.Error(suite.T(), err)
}
