package main

import (
	"log"
	"net/http"
	"github.com/gorilla/websocket"
)

func main() {
	go boatDataLiveMain()

	http.HandleFunc("/v1/ws", wsHandler)
	http.HandleFunc("/v1/ws/", wsHandler)

	log.Println("Ready...");
	http.ListenAndServe("127.0.0.1:8193", nil)
}

type ReqMsg struct {
	Cmd string `json:"cmd"`
	BoatKey string `json:"key"`
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	var upgrader = websocket.Upgrader {
		ReadBufferSize: 1024,
		WriteBufferSize: 1024,
		CheckOrigin: func (r *http.Request) bool { return true },
	}

	conn, err := upgrader.Upgrade(w, r, nil)

	if err != nil {
		log.Println(err)
		return
	}

	for {
		var req ReqMsg

		err := conn.ReadJSON(&req)
		if err != nil {
			log.Println(err)
			return
		}

		switch req.Cmd {
		case "bdl":
			wsReqBoatDataLive(&req, conn)
		default:
			log.Println("Invalid command: " + req.Cmd)
		}
	}
}
