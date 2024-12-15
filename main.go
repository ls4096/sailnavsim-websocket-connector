/**
 * Copyright (C) 2021-2024 ls4096 <ls4096@8bitbyte.ca>
 *
 * This program is free software: you can redistribute it and/or modify it
 * under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, version 3.
 *
 * This program is distributed in the hope that it will be useful, but WITHOUT
 * ANY WARRANTY; without even the implied warranty of MERCHANTABILITY or
 * FITNESS FOR A PARTICULAR PURPOSE. See the GNU General Public License for
 * more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program. If not, see <https://www.gnu.org/licenses/>.
 */

package main

import (
	"errors"
	"log"
	"net/http"
	"os"
	"github.com/gorilla/websocket"
)

func main() {
	log.Println("SailNavSim WebSocket Connector v1.3.0")

	listenHostPort, connectHostPort, err := parseArgs(os.Args[1:])
	if err != nil {
		log.Println(err)
		return
	}

	go boatDataLiveMain(connectHostPort)

	http.HandleFunc("/v1/ws", wsHandler)
	http.HandleFunc("/v1/ws/", wsHandler)

	log.Println("About to listen on " + listenHostPort + "...")

	err = http.ListenAndServe(listenHostPort, nil)
	if err != nil {
		log.Println(err)
	}
}

func parseArgs(args []string) (string, string, error) {
	if len(args) != 2 {
		return "", "", errors.New("ERROR: Program requires two arguments: listenHostPort, connectHostPort")
	}

	listenHostPort := args[0]
	connectHostPort := args[1]

	return listenHostPort, connectHostPort, nil
}

type ReqMsg struct {
	Cmd string `json:"cmd"`
	BoatKey string `json:"key"`
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	var upgrader = websocket.Upgrader {
		ReadBufferSize: 1024,
		WriteBufferSize: 4096,
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
		case "bdl": // "Boat data live" request
			wsReqBoatDataLive(&req, conn, false)
		case "bdl_g": // "Boat data live" request including nearby group members
			wsReqBoatDataLive(&req, conn, true)
		default:
			log.Println("Invalid command: " + req.Cmd)
		}
	}
}
