/**
 * Copyright (C) 2021 ls4096 <ls4096@8bitbyte.ca>
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
	"bufio"
	"container/list"
	"fmt"
	"log"
	"net"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"github.com/gorilla/websocket"
)


var _lock sync.Mutex

// Map of connections to boat keys (one connection can be associated with only one boat key)
var _conns = make(map[*websocket.Conn]string)

// Map of boat keys to lists of connections (one boat key can be associated with multiple connections)
var _keys = make(map[string]*list.List)

var _countConns int64 = 0
var _countMsgs int64 = 0

var _connectPort int = 0

var _boatKeyRegexp *regexp.Regexp = regexp.MustCompile("^[0-9a-f]{32}$")

const ITERATIONS_PER_LOG int64 = 60


func wsReqBoatDataLive(req *ReqMsg, conn *websocket.Conn) {
	if !_boatKeyRegexp.MatchString(req.BoatKey) {
		log.Println("Client sent invalid boat key!")
		conn.Close()
		return
	}

	_lock.Lock()
	defer _lock.Unlock()

	_, exists := _conns[conn]
	if !exists {
		// This is the first request on this connection, so associate it with the boat key.
		_conns[conn] = req.BoatKey
		_countConns++;
	} else {
		// Don't allow more than one boat key per connection.
		// If we encounter this situation, then just close the connection.
		conn.Close()
		return
	}

	// Add the connection to the list of connections that this boat key maps to.
	keyList, exists := _keys[req.BoatKey]
	if exists {
		keyList.PushBack(conn)
	} else {
		newList := list.New()
		newList.PushBack(conn)
		_keys[req.BoatKey] = newList
	}
}

type BoatDataLiveRespMsg struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
	Ctw float64 `json:"ctw"`
	Stw float64 `json:"stw"`
	Cog float64 `json:"cog"`
	Sog float64 `json:"sog"`
}

type KeyConnTuple struct {
	Key string
	Conn *websocket.Conn
}

func boatDataLiveMain(connectPort int) {
	_connectPort = connectPort

	var iterCount int64 = 0
	var iterTimeMin int64 = 999999999999
	var iterTimeMax int64 = -999999999999
	var iterTimeSum int64 = 0

	connsRemove := list.New()
	keysRemove := list.New()

	log.Println("Starting boat data live main loop...")

	// Main loop for live boat data.
	// Iterates approximately once every second (or slower, if things run longer).
	for {
		connsRemove.Init()
		keysRemove.Init()

		_lock.Lock()
		iterStartTime := time.Now()

		// Get the boat data responses from the simulator.
		resps := getBoatDataLiveResps()

		for boatKey, conns := range _keys {
			resp, exists := resps[boatKey]
			if !exists {
				// There was no valid response from the simulator for this boat key.
				log.Println("No response for boat key: " + boatKey)

				// Close this connection.
				for e := conns.Front(); e != nil; e = e.Next() {
					conn := e.Value.(*websocket.Conn)
					connsRemove.PushBack(conn)
					keysRemove.PushBack(KeyConnTuple { boatKey, conn })

					conn.Close()
				}

				continue
			}

			// For each connection in the list associated with this boat key,
			// send the boat data response message over the WebSocket.
			for e := conns.Front(); e != nil; e = e.Next() {
				conn := e.Value.(*websocket.Conn)
				err := conn.WriteJSON(resp)
				if err != nil {
					// Error sending message, so close this connection.
					log.Println(err)
					connsRemove.PushBack(conn)
					keysRemove.PushBack(KeyConnTuple { boatKey, conn })

					conn.Close()
				}
				_countMsgs++;
			}
		}

		// Remove closed connections from our tracking map.
		for e := connsRemove.Front(); e != nil; e = e.Next() {
			delete(_conns, e.Value.(*websocket.Conn))
		}

		// Remove closed connections from the list associated with our tracked boat keys map.
		for e := keysRemove.Front(); e != nil; e = e.Next() {
			kct := e.Value.(KeyConnTuple)
			connList, exists := _keys[kct.Key]
			if exists {
				for e2 := connList.Front(); e2 != nil; e2 = e2.Next() {
					if e2.Value.(*websocket.Conn) == kct.Conn {
						connList.Remove(e2)
						break // The connection will only be in the list once, so we're done.
					}
				}

				// If the boat has no more connections associated with it, then remove it from the map.
				if connList.Len() == 0 {
					delete(_keys, kct.Key)
				}
			}
		}

		// Measure and record iteration duration.
		iterTimeUs := time.Now().Sub(iterStartTime).Microseconds()
		if iterTimeUs < iterTimeMin {
			iterTimeMin = iterTimeUs
		}
		if iterTimeUs > iterTimeMax {
			iterTimeMax = iterTimeUs
		}
		iterTimeSum += iterTimeUs

		// Log some statistics periodically.
		if (iterCount > 0) && (iterCount % ITERATIONS_PER_LOG == 0) {
			log.Println("Now:        conns=" + strconv.Itoa(len(_conns)) + ", keys=" + strconv.Itoa(len(_keys)))
			log.Println("Cumulative: conns=" + strconv.FormatInt(_countConns, 10) + ", msgs=" + strconv.FormatInt(_countMsgs, 10))

			log.Println("Iteration times (min/avg/max us): " +
				strconv.FormatInt(iterTimeMin, 10) + "/" +
				strconv.FormatInt(iterTimeSum / ITERATIONS_PER_LOG, 10) + "/" +
				strconv.FormatInt(iterTimeMax, 10))

			// Reset iteration time counters.
			iterTimeMin = 999999999999
			iterTimeMax = -999999999999
			iterTimeSum = 0
		}

		iterCount++
		_lock.Unlock()
		time.Sleep(1 * time.Second)
	}
}

func getBoatDataLiveResps() map[string]BoatDataLiveRespMsg {
	resps := make(map[string]BoatDataLiveRespMsg)

	if len(_keys) == 0 {
		return resps
	}

	conn, err := net.Dial("tcp", "127.0.0.1:" + strconv.Itoa(_connectPort))
	if err != nil {
		log.Println(err)
		return resps
	}
	defer conn.Close()

	// For each boat key currently tracked, get the boat data from the simulator.
	for boatKey, _ := range _keys {
		fmt.Fprintf(conn, "bd_nc," + boatKey + "\n")
		line, err := bufio.NewReader(conn).ReadString('\n')
		if err != nil {
			log.Println(err)
			return resps
		}

		line = strings.Trim(line, "\n")
		if line == "error" {
			log.Println("Error returned from simulator when trying to get live data for boat key: " + boatKey)
			return resps
		}

		s := strings.Split(line, ",")
		switch s[2] {
		case "ok":
			lat, err := strconv.ParseFloat(s[3], 64)
			if err != nil {
				continue
			}

			lon, err := strconv.ParseFloat(s[4], 64)
			if err != nil {
				continue
			}

			ctw, err := strconv.ParseFloat(s[5], 64)
			if err != nil {
				continue
			}

			stw, err := strconv.ParseFloat(s[6], 64)
			if err != nil {
				continue
			}

			cog, err := strconv.ParseFloat(s[7], 64)
			if err != nil {
				continue
			}

			sog, err := strconv.ParseFloat(s[8], 64)
			if err != nil {
				continue
			}

			resps[s[1]] = BoatDataLiveRespMsg {
				Lat: lat,
				Lon: lon,
				Ctw: ctw,
				Stw: stw,
				Cog: cog,
				Sog: sog,
			}

		case "noboat":
			log.Println("No boat for key: " + boatKey)

		default:
			log.Println("Unexpected response from simulator: " + s[2])
		}
	}

	return resps
}
