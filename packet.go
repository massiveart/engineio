// (c) MASSIVE ART WebServices GmbH
//
// This source file is subject to the MIT license that is bundled
// with this source code in the file LICENSE.

package engineio

import (
	"bytes"
	"fmt"
	"strconv"
)

const (
	openID    = "0"
	closeID   = "1"
	pingID    = "2"
	pongID    = "3"
	messageID = "4"
	upgradeID = "5"
	noopID    = "6"

	// internal used types
	heartbeatID = "9"
)

var (
	okResponse     = []byte("ok")
	probeRequest   = []byte("2probe")
	probeResponse  = []byte("3probe")
	upgradeRequest = []byte("5")
)

type packet struct {
	connNum int64 // connection number frame bit
	index   int   // jsonp callback index (if used)
	Type    string
	Data    []byte
}

var (
	packetType = map[byte]string{
		'0': openID,
		'1': closeID,
		'2': pingID,
		'3': pongID,
		'4': messageID,
		'5': upgradeID,
		'6': noopID,
	}

	internalType = map[byte]string{
		'9': heartbeatID,
	}

	sep = []byte(":")
)

// decode decodes the polling payload. decode accepts packetType
// only.
func decode(data []byte) ([]packet, error) {
	packets := make([]packet, 0)

	for {
		i := bytes.Index(data, sep)
		if i == -1 {
			return nil, fmt.Errorf("short read")
		}
		n, err := strconv.Atoi(string(data[:i]))
		if err != nil {
			return nil, fmt.Errorf("ignoring payload")
		}

		data = data[i+1:]
		t, found := packetType[data[0]]
		if !found {
			return nil, fmt.Errorf("unknown packet type")
		}

		if len(data) < n {
			return nil, fmt.Errorf("malformed packet")
		}
		packets = append(packets, packet{
			Type: t,
			Data: data[1:n],
		})

		if len(data) == n {
			break
		}
		data = data[n:]
	}

	return packets, nil
}
