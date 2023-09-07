package wsclientengine

import (
	"github.com/go-playground/validator/v10"
)

// Defines configuration options for websocket connection.
//
// Use the factory function to get a new instance of the struct with nice defaults and then modify
// settings using With*** methods.
type WebsocketEngineConfigurationOptions struct {
	// Number of goroutine the engine will create to concurrently read messages, call user
	// callbacks and manage the shared websocket connection.
	//
	// Defaults to 4. Must be at least 1.
	ReaderRoutinesCount int `validate:"gte=1"`
	// If true, the engine will continuously try to reopen websocket connection when it
	// is interrupted by the server.
	//
	// Defaults to true.
	AutoReconnect bool
	// Base used to compute exponential reconnect retry delay (seconds).
	//
	// Defaults to 5s. Must be at least 1.
	AutoReconnectRetryDelayBaseSeconds int `validate:"gte=1"`
	// Maximum exponent used to compute exponential reconnect retry delay (inclusive).
	//
	// Defaults to 1. Must be at least 1.
	AutoReconnectRetryDelayMaxExponent int `validate:"gte=1"`
	// Delay to open websocket connection, call and complete OnOpen callback (milliseconds).
	//
	// Default to 300000 (5 minutes) - 0 disables the timeout.
	OnOpenTimeoutMs int64 `validate:"gte=0"`
	// Delay (milliseconds )to complete Stop() method. This includes triggering engine shutdown and
	// wait for the engine to stop: call & complete OnClose callback and close the connection.
	//
	// Default to 300000 (5 minutes) - 0 disables the timeout.
	StopTimeoutMs int64 `validate:"gte=0"`
}

// # Description
//
// Set opts.ReaderRoutinesCount and return the modified object. Method does not validate inputs.
//
// # ReaderRoutinesCount
//
// This option defines the number of goroutines websocket engine will span to concurrently manage
// the shared websocket connection and process messages.
//
// Defaults to 4. Must be greater or equal to 1.
//
// # Return
//
// The modified options.
func (opts *WebsocketEngineConfigurationOptions) WithReaderRoutinesCount(
	value int) *WebsocketEngineConfigurationOptions {
	// Set and return
	opts.ReaderRoutinesCount = value
	return opts
}

// # Description
//
// Set opts.AutoReconnect and return the modified object. The method does not validate inputs.
//
// # AutoReconnect
//
// This option defines whether the engine will automatically try to open a new connection to the
// websocket server in case the connection has been interrupted.
//
// Defaults to true (= enabled).
//
// # Return
//
// The modified options.
func (opts *WebsocketEngineConfigurationOptions) WithAutoReconnect(
	value bool) *WebsocketEngineConfigurationOptions {
	// Set and return
	opts.AutoReconnect = value
	return opts
}

// # Description
//
// Set opts.AutoReconnectRetryDelayBaseSeconds and return the modified object.
// The method does not validate inputs.
//
// # AutoReconnectRetryDelayBaseSeconds
//
// This option defines the number of seconds used as base in the exponential retry delay.
//
// Defaults to 5 seconds. Must be greater or equal to 1s.
//
// # Return
//
// The modified options.
func (opts *WebsocketEngineConfigurationOptions) WithAutoReconnectRetryDelayBaseSeconds(
	value int) *WebsocketEngineConfigurationOptions {
	// Set and return
	opts.AutoReconnectRetryDelayBaseSeconds = value
	return opts
}

// # Description
//
// Set opts.AutoReconnectRetryDelayMaxExponent and return the modified object.
// The method does not validate inputs.
//
// # AutoReconnectRetryDelayMaxExponent
//
// This option defines the number used as exponent in the exponential retry delay. If
// AutoReconnectRetryDelayBaseSeconds is 5s and AutoReconnectRetryDelayMaxExponent is 2, retry
// delay will be 5s^0 = 1s for first retry, 5s^1 = 5s for second retry and 5s^2 all other retries.
//
// Default to 1. Must be greater or equal to 1.
//
// # Return
//
// The modified options.
func (opts *WebsocketEngineConfigurationOptions) WithAutoReconnectRetryDelayMaxExponent(
	value int) *WebsocketEngineConfigurationOptions {
	// Set and return
	opts.AutoReconnectRetryDelayMaxExponent = value
	return opts
}

// # Description
//
// Set opts.OnOpenTimeoutMs and return the modified object.
// The method does not validate inputs.
//
// # OnOpenTimeoutMs
//
// This option defines the maximum delay (milliseconds) to open websocket connection, call and
// complete OnOpen callback. A value of 0 disables the timeout.
//
// Must be greater or equal to 0. Defaults to 5 minutes (= 300000).
//
// # Return
//
// The modified options.
func (opts *WebsocketEngineConfigurationOptions) WithOnOpenTimeoutMs(
	value int64) *WebsocketEngineConfigurationOptions {
	// Set and return
	opts.OnOpenTimeoutMs = value
	return opts
}

// # Description
//
// Set opts.StopTimeoutMs and return the modified object. The method does not validate inputs.
//
// # StopTimeoutMs
//
// This option defines the maximum delay (milliseconds) to stop websocket engine. A value of 0
// disables the timeout.
//
// Must be greater or equal to 0. Defaults to 5 minutes (= 300000).
//
// # Return
//
// The modified options.
func (opts *WebsocketEngineConfigurationOptions) WithStopTimeoutMs(
	value int64) *WebsocketEngineConfigurationOptions {
	// Set and return
	opts.StopTimeoutMs = value
	return opts
}

// # Description
//
// Factory which creates a new WebsocketEngineConfigurationOptions object with nice defaults.
// Settings can then be modified by the user by using With*** methods.
//
// # Default settings
//
//   - ReaderRoutinesCount = 4 , websocket engine will span 4 goroutines to concurrently
//     manage the shared websocket connection and process messages.
//   - AutoReconnect = true , websocket engine will continuously retry failed connection.
//   - AutoReconnectRetryDelayBaseSeconds = 5 , Exponential retry delay will use 5 seconds as base.
//   - AutoReconnectRetryDelayMaxExponent = 1 , Exponential retry delay will use 0 and then 1 as
//     exponent to compute the delay (5s^0 = 1s as delay on first retry, 5s^1 = 5s as next delays).
//   - OnOpenTimeoutMs = 300000 (5 minutes).
//   - StopTimeoutMs = 300000 (5 minutes).
func NewWebsocketEngineConfigurationOptions() *WebsocketEngineConfigurationOptions {
	return &WebsocketEngineConfigurationOptions{
		ReaderRoutinesCount:                4,
		AutoReconnect:                      true,
		AutoReconnectRetryDelayBaseSeconds: 5,
		AutoReconnectRetryDelayMaxExponent: 1,
		OnOpenTimeoutMs:                    300000,
		StopTimeoutMs:                      300000,
	}
}

// # Description
//
// Helper function which validates WebsocketEngineConfigurationOptions. Options are valid if:
//   - opts is not nil
//   - opts.ReaderRoutinesCount is greater or equal to 1
//   - opts.AutoReconnectRetryDelayBaseSeconds is greater or equal to 1
//   - opts.AutoReconnectRetryDelayMaxExponent is greater or equal to 1
//   - opts.OnOpenTimeoutMs is greater or equal to 0
//   - opts.StopTimeoutMs is greater or equal to 0
//
// # Returns
//
// InvalidValidationError for bad values passed in and nil or ValidationErrors as error otherwise.
// You will need to assert the error if it's not nil eg. err.(validator.ValidationErrors) to access
// the array of errors.
func Validate(opts *WebsocketEngineConfigurationOptions) error {
	// Validate
	return validator.New().Struct(opts)
}
