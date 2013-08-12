package engineio

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"
)

var (
	ErrUnknownSession = errors.New("unknown session id")
	ErrQueueFull      = errors.New("queue limit reached")
	ErrNotConnected   = errors.New("not connected")
)

// ServeMux is an HTTP request multiplexer. It matches the URL of each
// incoming request against a list of registered patterns and calls the
// handler for the pattern that most closely matches the URL.
type ServeMux struct {
	*http.ServeMux
	Addr   string
	Engine *EngineIO
}

// NewServeMux allocates and returns a new ServeMux. If config is nil,
// the DefaultConfig is used.
func NewServeMux(addr string, config *Config) *ServeMux {
	return &ServeMux{
		ServeMux: http.NewServeMux(),
		Addr:     addr,
		Engine:   NewEngineIO(config),
	}
}

// Close closes the engineio server and all it's connections.
func (s *ServeMux) Close() error {
	return s.Engine.Close()
}

// ListenAndServe listens on the TCP network address srv.Addr and then
// calls Serve to handle requests on incoming connections. If srv.Addr is
// blank, ":http" is used.
func (s *ServeMux) ListenAndServe() error {
	addr := s.Addr
	if addr == "" {
		addr = ":http"
	}

	s.HandleFunc(DefaultEngineioPath, s.Engine.Handler)

	return http.ListenAndServe(s.Addr, s)
}

// ConnectionFunc sets fn to be invoked when a new connection is
// established. It passes the established connection as an argument to
// the callback.
func (s *ServeMux) ConnectionFunc(fn func(Connection)) {
	s.Engine.connectionFunc = fn
}

// MessageFunc sets fn to be invoked when a message arrives. It passes
// the established connection along with the received message datai as
// arguments to the callback.
func (s *ServeMux) MessageFunc(fn func(Connection, []byte) error) {
	s.Engine.messageFunc = fn
}

// CloseFunc sets fn to be invoked when a session is considered to be
// lost. It passes the established connection as an argument to the
// callback. After disconnection the connection is considered to be
// destroyed, and it should not be used anymore.
func (s *ServeMux) CloseFunc(fn func(Connection)) {
	s.Engine.closeFunc = fn
}

// EngineIO handles transport abstraction and provide the user a handfull
// of callbacks to observe different events.
type EngineIO struct {
	sessions map[string]Connection
	remove   chan string
	config   *Config

	connectionFunc func(Connection)
	messageFunc    func(Connection, []byte) error
	closeFunc      func(Connection)
}

// NewEngineIO allocates and returns a new EngineIO. If config is nil,
// the DefaultConfig is used.
func NewEngineIO(config *Config) *EngineIO {
	e := &EngineIO{
		sessions: make(map[string]Connection),
		remove:   make(chan string),
	}

	if config == nil {
		e.config = DefaultConfig
	} else {
		e.config = config
	}

	go e.remover()
	return e
}

// Close closes the engineio server and all it's connections.
func (e EngineIO) Close() error {
	for _, conn := range e.sessions {
		conn.Close()
	}

	close(e.remove)
	return nil
}

func (e EngineIO) remover() {
	for {
		sid := <-e.remove
		delete(e.sessions, sid)
	}
}

// handshake returns a polling connection and an error if any.
// TODO: implement websocket handshake
func (e *EngineIO) handshake(w io.Writer, sid string) (Connection, error) {
	var payload = struct {
		Sid          string   `json:"sid"`
		Upgrades     []string `json:"upgrades"`
		PingInterval int64    `json:"pingInterval"`
		PingTimeout  int64    `json:"pingTimeout"`
	}{
		Sid:          sid,
		PingInterval: e.config.PingInterval,
		PingTimeout:  e.config.PingTimeout,
		Upgrades:     e.config.Upgrades,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	// TODO
	conn := &pollingConn{
		queue:        make(chan packet, e.config.QueueLength+maxHeartbeat),
		connections:  make(map[int64]*pollingWriter),
		connected:    true,
		sid:          sid,
		remove:       e.remove,
		pingInterval: time.Duration(e.config.PingInterval),
		queueLength:  e.config.QueueLength,
	}
	// polling queue flusher
	go conn.flusher()

	_, err = w.Write(conn.encode(packet{
		Type: openID,
		Data: data,
	}))
	if err != nil {
		return nil, err
	}

	return conn, nil
}

func (e *EngineIO) Handler(w http.ResponseWriter, req *http.Request) {
	sid := string(req.FormValue("sid"))

	switch uint(len(sid)) {
	case 0:
		sid = newSessionId()

		conn, err := e.handshake(w, sid)
		if err != nil {
			http.Error(w, "handshake: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// initialize function callbacks
		if e.connectionFunc != nil {
			e.connectionFunc(conn)
		}
		conn.messageFunc(e.messageFunc)
		conn.closeFunc(e.closeFunc)
		e.sessions[sid] = conn

	default:
		conn, found := e.sessions[sid]
		if !found {
			http.Error(w, ErrUnknownSession.Error(), http.StatusBadRequest)
			return
		}

		if upgrade := req.Header.Get("Upgrade"); upgrade == "websocket" {
			prevConn := conn
			newConn := &websocketConn{
				prevConn:    prevConn,
				sid:         sid,
				remove:      e.remove,
				pingTimeout: time.Duration(e.config.PingTimeout),
			}
			// initialize function callbacks
			newConn.closeFunc(e.closeFunc)
			newConn.messageFunc(e.messageFunc)

			if err := newConn.handle(w, req); err != nil {
				// got i/o timeout, remove session; we can't send
				// andy error message on a hijack'd connection.
				select {
				case e.remove <- sid:
				default:
				}
				return
			}

			return
		}

		// polling connection
		if err := conn.handle(w, req); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}