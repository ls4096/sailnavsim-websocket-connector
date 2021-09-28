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

var _conns = make(map[*websocket.Conn]string)
var _keys = make(map[string]*list.List)

var _iter int64 = 0
var _countConns int64 = 0
var _countMsgs int64 = 0

var _boatKeyRegexp *regexp.Regexp = regexp.MustCompile("^[0-9a-f]{32}$")


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
		_conns[conn] = req.BoatKey
		_countConns++;
	} else {
		// Don't allow more than one boat key per connection.
		conn.Close()
		return
	}

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
	connsRemove := list.New()
	keysRemove := list.New()

	for {
		connsRemove.Init()
		keysRemove.Init()

		_lock.Lock()

		resps := getBoatDataLiveResps(connectPort)

		for boatKey, conns := range _keys {
			resp, exists := resps[boatKey]
			if !exists {
				log.Println("No response for boat key: " + boatKey)

				for e := conns.Front(); e != nil; e = e.Next() {
					conn := e.Value.(*websocket.Conn)
					connsRemove.PushBack(conn)
					keysRemove.PushBack(KeyConnTuple { boatKey, conn })

					conn.Close()
				}

				continue
			}

			for e := conns.Front(); e != nil; e = e.Next() {
				conn := e.Value.(*websocket.Conn)
				err := conn.WriteJSON(resp)
				if err != nil {
					log.Println(err)
					connsRemove.PushBack(conn)
					keysRemove.PushBack(KeyConnTuple { boatKey, conn })

					conn.Close()
				}
				_countMsgs++;
			}
		}

		for e := connsRemove.Front(); e != nil; e = e.Next() {
			delete(_conns, e.Value.(*websocket.Conn))
		}

		for e := keysRemove.Front(); e != nil; e = e.Next() {
			kct := e.Value.(KeyConnTuple)
			l, exists := _keys[kct.Key]
			if exists {
				for e2 := l.Front(); e2 != nil; e2 = e2.Next() {
					if e2.Value.(*websocket.Conn) == kct.Conn {
						l.Remove(e2)
						break
					}
				}
			}

			if l.Len() == 0 {
				delete(_keys, kct.Key)
			}
		}

		if _iter % 60 == 0 {
			log.Println("Now:        conns=" + strconv.Itoa(len(_conns)) + ", keys=" + strconv.Itoa(len(_keys)))
			log.Println("Cumulative: conns=" + strconv.FormatInt(_countConns, 10) + ", msgs=" + strconv.FormatInt(_countMsgs, 10))
		}
		_iter++

		_lock.Unlock()
		time.Sleep(1 * time.Second)
	}
}

func getBoatDataLiveResps(connectPort int) map[string]BoatDataLiveRespMsg {
	resps := make(map[string]BoatDataLiveRespMsg)

	conn, err := net.Dial("tcp", "127.0.0.1:" + strconv.Itoa(connectPort))
	if err != nil {
		log.Println(err)
		return resps
	}
	defer conn.Close()

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
