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
	Type    string
	Data    []byte
}

var packetType = map[byte]string{
	'0': openID,
	'1': closeID,
	'2': pingID,
	'3': pongID,
	'4': messageID,
	'5': upgradeID,
	'6': noopID,
}

var internalType = map[byte]string{
	'9': heartbeatID,
}

// decode decodes the polling payload. decode accepts packetType
// only.
func decode(data []byte) ([]packet, error) {
	packets := make([]packet, 0)

	for {
		elems := bytes.Split(data, []byte(":"))
		m := len(elems)
		if len(elems) < 2 {
			return nil, fmt.Errorf("short read")
		}
		n, err := strconv.Atoi(string(elems[0]))
		if err != nil {
			return nil, fmt.Errorf("ignoring payload")
		}

		t, found := packetType[elems[1][0]]
		if !found {
			return nil, fmt.Errorf("unknown packet type")
		}

		packets = append(packets, packet{
			Type: t,
			Data: elems[1][1:n],
		})

		if m > 2 {
			data = data[len(elems[0])+n+1:]
		} else {
			break
		}
	}

	return packets, nil
}
