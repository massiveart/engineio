// (c) MASSIVE ART WebServices GmbH
//
// This source file is subject to the MIT license that is bundled
// with this source code in the file LICENSE.

package engineio

const DefaultEngineioPath = "/engine.io/"

type Config struct {
	// Maximum amount of messages to store for a connection. If a
	// connection has QueueLength amount of undelivered messages,
	// the following writes will return ErrQueueFull error.
	QueueLength int

	// The size of the read buffer in bytes.
	ReadBufferSize int

	// Ping/Ping interval in milliseconds.
	PingInterval int64

	// Ping timeout in milliseconds.
	PingTimeout int64

	// Upgrades to use. (Only websocket supported).
	Upgrades []string
}

var DefaultConfig = &Config{
	QueueLength:  10,
	PingInterval: 25000,
	PingTimeout:  60000,
	Upgrades:     []string{"websocket"},
}
