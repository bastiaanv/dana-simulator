package main

import (
	"dana/simulator/server"
	"net/http"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{}

var s = server.NewSimulator()

func main() {
	s.StartBluetooth()
	// http.HandleFunc("/ws", handleWS)
	http.ListenAndServe(":3003", nil)
}

// func handleWS(w http.ResponseWriter, r *http.Request) {
// 	// Upgrade upgrades the HTTP server connection to the WebSocket protocol.
// 	conn, err := upgrader.Upgrade(w, r, nil)
// 	if err != nil {
// 		log.Print("upgrade failed: ", err)
// 		return
// 	}
// 	defer conn.Close()

// 	// Continuosly read and write message
// 	for {
// 		mt, message, err := conn.ReadMessage()
// 		if err != nil {
// 			log.Println("read failed:", err)
// 			break
// 		}
// 		input := string(message)
// 	}
// }

// func startPump() {
// 	if s.State.Status == server.STATUS_RUNNING {
// 		return
// 	}

// 	s.StartBluetooth()
// }
