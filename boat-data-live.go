/**
 * Copyright (C) 2021-2023 ls4096 <ls4096@8bitbyte.ca>
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
	"math"
	"net"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"github.com/gorilla/websocket"
)


var _lock sync.Mutex

type BoatInfo struct {
	BoatKey string
	FriendlyName string
}

// Map of connections to boat keys (one connection can be associated with only one boat key)
type ConnCtx struct {
	BoatKey string
	GroupBoats *list.List
}
var _conns = make(map[*websocket.Conn]ConnCtx)

// Map of boat keys to list of connections (one boat key can be associated with multiple connections)
var _keys = make(map[string]*list.List)

// Tracker for boat keys in groups
type TrackedBoatEntry struct {
	BoatKey string
	RefCount uint64
}
var _trackedBoats = make(map[string]*TrackedBoatEntry)

var _countConns int64 = 0
var _countMsgs int64 = 0

var _connectPort int = 0

var _boatKeyRegexp *regexp.Regexp = regexp.MustCompile("^[0-9a-f]{32}$")

const ITERATIONS_PER_LOG int64 = 60

const DIAL_TIMEOUT = 3 * time.Second
const CONN_RW_TIMEOUT = 3 * time.Second


func wsReqBoatDataLive(req *ReqMsg, conn *websocket.Conn, withGroup bool) {
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

		if withGroup {
			// Request to include nearby boats in group
			groupBoats := getBoatsInGroup(req.BoatKey)
			if groupBoats == nil {
				conn.Close()
				return
			}

			_conns[conn] = ConnCtx {
				BoatKey: req.BoatKey,
				GroupBoats: groupBoats,
			}

			trackBoats(groupBoats)
		} else {
			// Request to include only this boat
			_conns[conn] = ConnCtx {
				BoatKey: req.BoatKey,
				GroupBoats: nil,
			}

			trackBoat(req.BoatKey)
		}

		_countConns++
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

type BoatGroupRespMsg struct {
	ThisBoat BoatDataLiveRespMsg `json:"you"`
	OtherBoats map[string][3]float64 `json:"others"`
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
				connCtx := _conns[conn]
				closeConn := false
				if connCtx.GroupBoats != nil {
					// Create the response message for this boat plus the other boats in the same group.
					resp := createBoatGroupRespMsg(&connCtx, resps)
					err := conn.WriteJSON(resp)
					if err != nil {
						log.Println(err)
						closeConn = true
					}
				} else {
					err := conn.WriteJSON(resp)
					if err != nil {
						log.Println(err)
						closeConn = true
					}
				}

				if closeConn {
					// Error sending message, so close this connection.
					connsRemove.PushBack(conn)
					keysRemove.PushBack(KeyConnTuple { boatKey, conn })

					conn.Close()
				}

				_countMsgs++
			}
		}

		// Remove closed connections from our tracking map.
		for e := connsRemove.Front(); e != nil; e = e.Next() {
			connCtx := _conns[e.Value.(*websocket.Conn)]
			if connCtx.GroupBoats != nil {
				untrackBoats(connCtx.GroupBoats)
			} else {
				untrackBoat(connCtx.BoatKey)
			}

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
		iterTimeDuration := time.Now().Sub(iterStartTime)
		iterTimeUs := iterTimeDuration.Microseconds()
		if iterTimeUs < iterTimeMin {
			iterTimeMin = iterTimeUs
		}
		if iterTimeUs > iterTimeMax {
			iterTimeMax = iterTimeUs
		}
		iterTimeSum += iterTimeUs

		// Log some statistics periodically.
		if (iterCount > 0) && (iterCount % ITERATIONS_PER_LOG == 0) {
			log.Println("Now:        conns=" + strconv.Itoa(len(_conns)) + ", keys=" + strconv.Itoa(len(_keys)) + ", tracked=" + strconv.Itoa(len(_trackedBoats)))
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
		time.Sleep(time.Second - iterTimeDuration)
	}
}

func getBoatDataLiveResps() map[string]BoatDataLiveRespMsg {
	resps := make(map[string]BoatDataLiveRespMsg)

	if len(_keys) == 0 {
		return resps
	}

	conn, err := net.DialTimeout("tcp", "127.0.0.1:" + strconv.Itoa(_connectPort), DIAL_TIMEOUT)
	if err != nil {
		log.Println(err)
		return resps
	}
	defer conn.Close()

	if conn.SetDeadline(time.Now().Add(CONN_RW_TIMEOUT)) != nil {
		log.Println(err)
		return resps
	}

	requestWriterDone := make(chan int)
	go func() {
		for boatKey, _ := range _trackedBoats {
			fmt.Fprintf(conn, "bd_nc," + boatKey + "\n")
		}

		requestWriterDone <- 0
	}()

	responseReader := bufio.NewReader(conn)

	// For each boat key currently tracked, get the boat data from the simulator.
	numTracked := len(_trackedBoats)
	for i := 0; i < numTracked; i++ {
		line, err := responseReader.ReadString('\n')

		if err != nil {
			log.Println(err)
			break
		}

		line = strings.Trim(line, "\n")
		if line == "error" {
			log.Println("Error returned from simulator when trying to get live data for boat num: " + strconv.Itoa(i))
			break
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
			log.Println("No boat for key: " + s[1])

		default:
			log.Println("Unexpected response from simulator: " + s[2])
		}
	}

	// Ensure that our request writer goroutine has finished before continuing.
	<-requestWriterDone

	return resps
}

func getBoatsInGroup(boatKey string) *list.List {
	conn, err := net.DialTimeout("tcp", "127.0.0.1:" + strconv.Itoa(_connectPort), DIAL_TIMEOUT)
	if err != nil {
		log.Println(err)
		return nil
	}
	defer conn.Close()

	if conn.SetDeadline(time.Now().Add(CONN_RW_TIMEOUT)) != nil {
		log.Println(err)
		return nil
	}

	groupKeys := list.New()

	fmt.Fprintf(conn, "boatgroupmembers," + boatKey + "\n")
	start := true
	reader := bufio.NewReader(conn)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			log.Println(err)
			return nil
		}

		line = strings.Trim(line, "\n")

		if start {
			if line == "error" {
				log.Println("Error returned from simulator when trying to get boat group membership for boat key: " + boatKey)
				return nil
			}

			s := strings.Split(line, ",")
			switch s[2] {
			case "ok":
				start = false
				continue

			default:
				log.Println("Unexpected code (\"" + s[2] + "\") returned from simulator when trying to get boat group membership for boat key: " + boatKey)
				return nil
			}
		} else if line == "" {
			return groupKeys
		} else {
			s := strings.Split(line, ",")
			if s[1] != "!" {
				groupKeys.PushBack(&BoatInfo {
					BoatKey: s[0],
					FriendlyName: s[1],
				})
			}
		}
	}
}

func trackBoats(boats *list.List) {
	for boat := boats.Front(); boat != nil; boat = boat.Next() {
		trackBoat(boat.Value.(*BoatInfo).BoatKey)
	}
}

func untrackBoats(boats *list.List) {
	for boat := boats.Front(); boat != nil; boat = boat.Next() {
		untrackBoat(boat.Value.(*BoatInfo).BoatKey)
	}
}

func trackBoat(boatKey string) {
	entry, exists := _trackedBoats[boatKey]
	if !exists {
		_trackedBoats[boatKey] = &TrackedBoatEntry {
			BoatKey: boatKey,
			RefCount: 1,
		}
	} else {
		entry.RefCount++
	}
}

func untrackBoat(boatKey string) {
	entry, exists := _trackedBoats[boatKey]
	if exists {
		entry.RefCount--
		if entry.RefCount == 0 {
			delete(_trackedBoats, boatKey)
		}
	}
}

func createBoatGroupRespMsg(connCtx *ConnCtx, resps map[string]BoatDataLiveRespMsg) *BoatGroupRespMsg {
	others := make(map[string][3]float64)

	thisBoatData := resps[connCtx.BoatKey]

	// Iterate through all the other boats in the same group to see which should be included in the response message.
	for e := connCtx.GroupBoats.Front(); e != nil; e = e.Next() {
		otherBoatKey := e.Value.(*BoatInfo).BoatKey
		friendlyName := e.Value.(*BoatInfo).FriendlyName

		otherBoatData := resps[otherBoatKey]

		if connCtx.BoatKey == otherBoatKey {
			continue // Our boat, so don't include it here.
		}

		dist := roughCloseDistance(thisBoatData.Lat, thisBoatData.Lon, otherBoatData.Lat, otherBoatData.Lon)

		if dist > 15.0 {
			continue // Other boat too far away (more than 15 NM) to see live, so don't include it.
		}

		others[friendlyName] = [3]float64 { otherBoatData.Lat, otherBoatData.Lon, roundCourse(otherBoatData.Ctw, dist) }
	}

	return &BoatGroupRespMsg {
		ThisBoat: resps[connCtx.BoatKey],
		OtherBoats: others,
	}
}

func roughCloseDistance(localLat float64, localLon float64, otherLat float64, otherLon float64) float64 {
	latMid := (localLat + otherLat) / 2.0
	if latMid > 89.0 {
		latMid = 89.0
	} else if latMid < -89.0 {
		latMid = -89.0
	}

	yDiff := 60.0 * math.Abs(localLat - otherLat)
	if yDiff > 60.0 {
		return 60.0
	}

	nmPerLonDeg := 60.0 * math.Cos(latMid * math.Pi / 180.0)

	xDiff := nmPerLonDeg * diffLon(localLon, otherLon)
	if xDiff > 60.0 {
		return 60.0
	}

	return math.Sqrt(xDiff * xDiff + yDiff * yDiff)
}

func diffLon(a float64, b float64) float64 {
	diff := math.Abs(a - b)
	if diff > 180.0 {
		if a < b {
			diff = math.Abs(a + 360.0 - b)
		} else {
			diff = math.Abs(b + 360.0 - a)
		}
	}

	return diff
}

func roundCourse(course float64, distance float64) float64 {
	if distance >= 6.0 {
		return math.Round(course / 22.5) * 22.5 // To nearest 22.5 deg (16 points)
	} else if distance >= 3.0 {
		return math.Round(course / 11.25) * 11.25 // To nearest 11.25 deg (32 points)
	} else {
		return math.Round(course / 5.625) * 5.625 // To nearest 5.625 deg (64 points)
	}
}
