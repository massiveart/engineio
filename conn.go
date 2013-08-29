// (c) MASSIVE ART WebServices GmbH
//
// This source file is subject to the MIT license that is bundled
// with this source code in the file LICENSE.

package engineio

import (
	"io"
	"net/http"
)

type Connection interface {
	io.WriteCloser
	ID() string

	upgrade(packet) error
	encode(packet) []byte
	handle(http.ResponseWriter, *http.Request) error

	messageFunc(func(Connection, []byte) error)
	closeFunc(func(Connection))
}
