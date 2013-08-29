// +build ignore

package main

import (
	"log"
	"net/http"

	"github.com/massiveart/engineio"
)

var page = []byte(`
<!DOCTYPE html>
<html class="no-js">
<body>
	<div>
		<form onsubmit="socket.send($('#message').val()); $('#message').val(''); return false;">
			<input id="message" type="text" placeholder="Message ...">
			<button id="send" type="submit">Send</button>
		</form>
	</div>
	<div id="messages"></div>
	<div id="status"></div>
	
	<script src="//ajax.googleapis.com/ajax/libs/jquery/1.9.1/jquery.min.js"></script>
	<script src="https://rawgithub.com/LearnBoost/engine.io-client/master/engine.io.js"></script>
	<script>
	var socket = new eio.Socket('ws://localhost:9090', {
		transports: ['polling', 'websocket']
	});
	socket.on('open', function () {
		socket.on('message', function (data) {
			$('#messages').append('<div>' + data + '</div>');
		});
		socket.on('close', function() {
			$('#status').append('closed');
		});
	});
	</script>
</body>
</html>
`)

func main() {
	enio := engineio.NewEngineIO(nil)
	defer enio.Close()

	enio.ConnectionFunc(func(conn engineio.Connection) {})

	enio.MessageFunc(func(conn engineio.Connection, data []byte) error {
		_, err := conn.Write(data)
		return err
	})

	enio.CloseFunc(func(conn engineio.Connection) {})

	server := http.NewServeMux()
	server.HandleFunc(engineio.DefaultEngineioPath, enio.Handler)
	server.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		w.Write(page)
	})

	log.Fatal(http.ListenAndServe("localhost:9090", server))
}
