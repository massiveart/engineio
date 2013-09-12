// (c) MASSIVE ART WebServices GmbH
//
// This source file is subject to the MIT license that is bundled
// with this source code in the file LICENSE.

package engineio

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"sync"
	"time"
)

const maxHeartbeat = 10

type pollingWriter struct {
	w         io.Writer
	connected bool // indicates if the writer is ready for writing
	done      chan<- bool
}

func (w *pollingWriter) Write(p []byte) (int, error) {
	defer func() {
		w.done <- true
	}()

	if !w.connected {
		return 0, ErrNotConnected
	}

	return w.w.Write(p)
}

type pollingConn struct {
	mu   sync.RWMutex // protects the connections queue/map
	rwmu sync.Mutex   // protects the queue

	sid         string
	queue       chan packet
	connected   bool // indicates if the connection has been disconnected
	upgraded    bool // indicates if the connection has been upgraded
	index       int  // jsonp callback index (if jsonp is used)
	connNum     int64
	connections map[int64]*pollingWriter

	remove       chan<- string
	pingInterval time.Duration
	queueLength  int

	messageFn func(Connection, []byte) error
	closeFn   func(Connection)
}

// TODO: handle read/write timeout
//func (c *pollingConn) reader(dst io.Writer, src io.Reader) (err error) {
func (c *pollingConn) reader(dst io.Writer, req *http.Request) (err error) {
	data := []byte{}
	if c.index == -1 {
		data, err = ioutil.ReadAll(req.Body)
		if err != nil {
			return err
		}
	} else {
		data = []byte(req.FormValue("d"))
	}

	packets, err := decode(data)
	if err != nil {
		return err
	}

	for _, p := range packets {
		switch p.Type {
		case closeID:
			return nil

		case pingID:
			_, err = dst.Write(c.encode(packet{
				index: c.index,
				Type:  pongID,
			}))
			if err != nil {
				return err
			}

		case messageID:
			if c.messageFunc != nil {
				if err = c.messageFn(c, p.Data); err != nil {
					// TODO
				}
			}

			if _, err = dst.Write(okResponse); err != nil {
				return err
			}
		}
	}
	return nil

}

func (c *pollingConn) handle(w http.ResponseWriter, req *http.Request) (err error) {
	if req.Method == "POST" {
		return c.reader(w, req)
	}

	// add a fresh polling writer
	c.mu.Lock()

	closeNotifier := w.(http.CloseNotifier).CloseNotify()
	done := make(chan bool)
	c.connNum++
	num := c.connNum

	c.connections[c.connNum] = &pollingWriter{
		w:         w,
		done:      done,
		connected: true,
	}

	c.mu.Unlock()

	// handle write done, timeout or closed by user signals
Loop:
	for {
		select {
		case <-done:
			break Loop

		case <-closeNotifier:
			c.Close()
			break Loop

		case <-time.After(c.pingInterval * time.Millisecond):
			select {
			case c.queue <- packet{
				connNum: num,
				index:   c.index,
				Type:    pongID,
			}:
			default:
			}
		}
	}

	// delete polling writer
	c.mu.Lock()
	c.connections[num].connected = false
	delete(c.connections, num)
	c.mu.Unlock()

	return
}

func (c *pollingConn) ID() string {
	return c.sid
}

func (c *pollingConn) Write(data []byte) (int, error) {
	c.rwmu.Lock()
	defer c.rwmu.Unlock()

	if !c.connected {
		return 0, ErrNotConnected
	}

	select {
	case c.queue <- packet{
		index: c.index,
		Type:  messageID,
		Data:  data,
	}:

	default:
		return 0, ErrQueueFull
	}
	return len(data), nil
}

func (c *pollingConn) Close() error {
	c.rwmu.Lock()
	defer c.rwmu.Unlock()

	c.connected = false
	close(c.queue)

	if !c.upgraded {
		select {
		case c.remove <- c.sid:
		default:
		}

		if c.closeFn != nil {
			c.closeFn(c)
		}
	}

	return nil
}

func (c *pollingConn) upgrade(p packet) error {
	c.rwmu.Lock()
	defer func() {
		c.upgraded = true
		c.rwmu.Unlock()
	}()

	if !c.connected {
		return ErrNotConnected
	}

	p.index = c.index
	select {
	case c.queue <- p:

	default:
		return ErrQueueFull
	}
	return nil
}

func (c *pollingConn) encode(p packet) []byte {
	data := append([]byte(p.Type), p.Data...)

	if c.index != -1 {
		ndata := fmt.Sprintf("%d:", len(data))
		data = append([]byte(ndata), data...)
		ndata = fmt.Sprintf("___eio[%d](%q);", c.index, data)
		return []byte(ndata)
	}

	ndata := fmt.Sprintf("%d:", len(data))
	return append([]byte(ndata), data...)
}

func (c *pollingConn) flusher() {
	buf := bytes.NewBuffer(nil)
	heartbeats := 0

	for p := range c.queue {
		num := int64(-1)

		switch p.Type {
		case heartbeatID:
			time.Sleep(500 * time.Millisecond)
			heartbeats++
		case pongID:
			num = p.connNum
			buf.Write(c.encode(p))
		default:
			buf.Write(c.encode(p))
		}
		n := 1

	DrainLoop:
		for n < c.queueLength {
			select {
			case p = <-c.queue:
				n++
				switch p.Type {
				case heartbeatID:
					time.Sleep(500 * time.Millisecond)
					heartbeats++
				case pongID:
					num = p.connNum
					buf.Write(c.encode(p))
				default:
					buf.Write(c.encode(p))
				}

			default:
				break DrainLoop
			}
		}

		c.mu.RLock()
		var writer *pollingWriter
		if num != -1 {
			writer = c.connections[num]
		} else { // get first writer
			for _, writer = range c.connections {
				break
			}
		}
		c.mu.RUnlock()

		if writer != nil {
			if _, err := writer.Write(buf.Bytes()); err != nil {
				c.Close()
				return
			}
			buf.Reset()
			heartbeats = 0
		} else {
			if heartbeats >= maxHeartbeat-1 {
				c.Close()
				return
			}

			c.rwmu.Lock()
			if !c.connected {
				c.rwmu.Unlock()
				return
			}
			select {
			case c.queue <- packet{index: c.index, Type: heartbeatID}:

			default:
			}
			c.rwmu.Unlock()
		}
	}
}

func (c *pollingConn) messageFunc(fn func(Connection, []byte) error) {
	c.messageFn = fn
}

func (c *pollingConn) closeFunc(fn func(Connection)) {
	c.closeFn = fn
}
