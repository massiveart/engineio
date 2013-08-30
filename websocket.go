// (c) MASSIVE ART WebServices GmbH
//
// This source file is subject to the MIT license that is bundled
// with this source code in the file LICENSE.

package engineio

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"time"

	"code.google.com/p/go.net/websocket"
)

type websocketConn struct {
	conn     *websocket.Conn
	prevConn Connection

	ready           chan bool
	closeConnection chan bool

	sid         string
	remove      chan<- string
	pingTimeout time.Duration

	messageFn func(Connection, []byte) error
	closeFn   func(Connection)
}

func (c *websocketConn) handle(w http.ResponseWriter, req *http.Request) (err error) {
	c.ready = make(chan bool)
	c.closeConnection = make(chan bool)

	connection := func(conn *websocket.Conn) {
		c.conn = conn

		buf := make([]byte, 6)
		if _, err = c.conn.Read(buf); err != nil {
			return
		}
		if bytes.Compare(buf[0:6], probeRequest) != 0 {
			err = errors.New("unknown probe message: " + string(buf))
			return
		}

		if _, err = c.conn.Write(probeResponse); err != nil {
			return
		}

		// Upgrade previous (polling) connection.
		if err = c.prevConn.upgrade(packet{Type: noopID}); err != nil {
			err = errors.New("cannot upgrade connection")
			return
		}

		if _, err = c.conn.Read(buf); err != nil {
			return
		}
		if bytes.Compare(buf[0:1], upgradeRequest) != 0 {
			err = errors.New("unknown upgrade message: " + string(buf))
			return
		}

		// Now the connection is upgraded and we can safely close the
		// queue channel and the connection.
		c.prevConn.Close()

		now := time.Now()
		// reset read deadline
		err = c.conn.SetReadDeadline(now.Add(c.pingTimeout * time.Millisecond))
		if err != nil {
			return
		}
		// reset write deadline
		err = c.conn.SetWriteDeadline(now.Add(c.pingTimeout * time.Millisecond))
		if err != nil {
			return
		}

		c.ready <- true
		<-c.closeConnection
	}

	go websocket.Handler(connection).ServeHTTP(w, req)

	<-c.ready

	return c.reader()
}

func (c *websocketConn) ID() string {
	return c.sid
}

func (c *websocketConn) Write(data []byte) (int, error) {
	defer func() {
		// reset write deadline
		now := time.Now()
		c.conn.SetWriteDeadline(now.Add(c.pingTimeout * time.Millisecond))
	}()

	packet := packet{Type: messageID, Data: data}
	return c.conn.Write(c.encode(packet))
}

// upgrade is a noop on websocket connections.
func (c *websocketConn) upgrade(p packet) error {
	return errors.New("websocket upgrade is a noop")
}

func (c *websocketConn) Close() error {
	c.remove <- c.sid
	c.closeConnection <- true

	if c.closeFn != nil {
		c.closeFn(c)
	}

	return c.conn.Close()
}

func (c *websocketConn) encode(p packet) []byte {
	return []byte(fmt.Sprintf("%s%s", p.Type, p.Data))
}

// reader closes if a read or write error happens.
func (c *websocketConn) reader() (err error) {
	defer c.Close()

	var data []byte
	for {
		if err = websocket.Message.Receive(c.conn, &data); err != nil {
			return
		}
		// reset read deadline
		now := time.Now()
		err = c.conn.SetReadDeadline(now.Add(c.pingTimeout * time.Millisecond))
		if err != nil {
			return
		}

		p := packet{
			Type: string(data[:1]),
			Data: data[1:],
		}
		switch p.Type {
		case closeID:
			return

		case pingID:
			_, err = c.conn.Write(c.encode(packet{Type: pongID}))
			if err != nil {
				return
			}

			// reset write deadline
			now = time.Now()
			err = c.conn.SetWriteDeadline(now.Add(c.pingTimeout * time.Millisecond))
			if err != nil {
				return
			}

		case messageID:
			if c.messageFn != nil {
				if err = c.messageFn(c, p.Data); err != nil {
					return
				}
			}
		}
	}
}

func (c *websocketConn) messageFunc(fn func(Connection, []byte) error) {
	c.messageFn = fn
}

func (c *websocketConn) closeFunc(fn func(Connection)) {
	c.closeFn = fn
}
