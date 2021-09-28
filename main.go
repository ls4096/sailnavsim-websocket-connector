package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"github.com/gorilla/websocket"
)

func main() {
	log.Println("SailNavSim WebSocket Connector v1.0.0")

	listenPort, connectPort, err := parseArgs(os.Args[1:])
	if err != nil {
		log.Println(err)
		return
	}

	go boatDataLiveMain(connectPort)

	http.HandleFunc("/v1/ws", wsHandler)
	http.HandleFunc("/v1/ws/", wsHandler)

	log.Println("About to listen on localhost port " + strconv.Itoa(listenPort) + "...")

	err = http.ListenAndServe("127.0.0.1:" + strconv.Itoa(listenPort), nil)
	if err != nil {
		log.Println(err)
	}
}

func parseArgs(args []string) (int, int, error) {
	if len(args) != 2 {
		return -1, -1, errors.New("ERROR: Program requires two arguments: listenPort, connectPort")
	}

	listenPort, err := strconv.Atoi(args[0])
	if err != nil {
		return -1, -1, fmt.Errorf("Failed to parse listen port: %w", err)
	}

	connectPort, err := strconv.Atoi(args[1])
	if err != nil {
		return -1, -1, fmt.Errorf("Failed to parse connect port: %w", err)
	}

	return listenPort, connectPort, nil
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
