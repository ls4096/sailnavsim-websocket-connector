/**
 * Copyright (C) 2022 ls4096 <ls4096@8bitbyte.ca>
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
	"math"
	"math/rand"
	"testing"
)


func TestRoughCloseDistance(t *testing.T) {
	FACTOR_LAT_30 := math.Cos(30.0 * math.Pi / 180.0)
	FACTOR_LAT_45 := math.Cos(45.0 * math.Pi / 180.0)
	FACTOR_LAT_60 := math.Cos(60.0 * math.Pi / 180.0)


	roughCloseDistanceChecks(t, 0.0, 0.0, 0.0, 0.5, 30.0)
	roughCloseDistanceChecks(t, 0.0, 170.0, 0.0, 170.5, 30.0)


	// Across 180 longitude meridian
	roughCloseDistanceChecks(t, 0.0, 179.0, 0.0, -179.0, 120.0)
	roughCloseDistanceChecks(t, 0.0, 179.5, 0.0, -179.5, 60.0)
	roughCloseDistanceChecks(t, 0.0, 179.75, 0.0, -179.75, 30.0)
	roughCloseDistanceChecks(t, 0.0, 179.875, 0.0, -179.875, 15.0)

	roughCloseDistanceChecks(t, 30.0, 179.0, 30.0, -179.0, 120.0 * FACTOR_LAT_30)
	roughCloseDistanceChecks(t, 30.0, 179.5, 30.0, -179.5, 60.0 * FACTOR_LAT_30)
	roughCloseDistanceChecks(t, 30.0, 179.75, 30.0, -179.75, 30.0 * FACTOR_LAT_30)
	roughCloseDistanceChecks(t, 30.0, 179.875, 30.0, -179.875, 15.0 * FACTOR_LAT_30)

	roughCloseDistanceChecks(t, 45.0, 179.0, 45.0, -179.0, 120.0 * FACTOR_LAT_45)
	roughCloseDistanceChecks(t, 45.0, 179.5, 45.0, -179.5, 60.0 * FACTOR_LAT_45)
	roughCloseDistanceChecks(t, 45.0, 179.75, 45.0, -179.75, 30.0 * FACTOR_LAT_45)
	roughCloseDistanceChecks(t, 45.0, 179.875, 45.0, -179.875, 15.0 * FACTOR_LAT_45)

	roughCloseDistanceChecks(t, 60.0, 179.0, 60.0, -179.0, 120.0 * FACTOR_LAT_60)
	roughCloseDistanceChecks(t, 60.0, 179.5, 60.0, -179.5, 60.0 * FACTOR_LAT_60)
	roughCloseDistanceChecks(t, 60.0, 179.75, 60.0, -179.75, 30.0 * FACTOR_LAT_60)
	roughCloseDistanceChecks(t, 60.0, 179.875, 60.0, -179.875, 15.0 * FACTOR_LAT_60)


	// Across prime meridian
	roughCloseDistanceChecks(t, 0.0, 1.0, 0.0, -1.0, 120.0)
	roughCloseDistanceChecks(t, 0.0, 0.5, 0.0, -0.5, 60.0)
	roughCloseDistanceChecks(t, 0.0, 0.25, 0.0, -0.25, 30.0)
	roughCloseDistanceChecks(t, 0.0, 0.125, 0.0, -0.125, 15.0)

	roughCloseDistanceChecks(t, 30.0, 1.0, 30.0, -1.0, 120.0 * FACTOR_LAT_30)
	roughCloseDistanceChecks(t, 30.0, 0.5, 30.0, -0.5, 60.0 * FACTOR_LAT_30)
	roughCloseDistanceChecks(t, 30.0, 0.25, 30.0, -0.25, 30.0 * FACTOR_LAT_30)
	roughCloseDistanceChecks(t, 30.0, 0.125, 30.0, -0.125, 15.0 * FACTOR_LAT_30)

	roughCloseDistanceChecks(t, 45.0, 1.0, 45.0, -1.0, 120.0 * FACTOR_LAT_45)
	roughCloseDistanceChecks(t, 45.0, 0.5, 45.0, -0.5, 60.0 * FACTOR_LAT_45)
	roughCloseDistanceChecks(t, 45.0, 0.25, 45.0, -0.25, 30.0 * FACTOR_LAT_45)
	roughCloseDistanceChecks(t, 45.0, 0.125, 45.0, -0.125, 15.0 * FACTOR_LAT_45)

	roughCloseDistanceChecks(t, 60.0, 1.0, 60.0, -1.0, 120.0 * FACTOR_LAT_60)
	roughCloseDistanceChecks(t, 60.0, 0.5, 60.0, -0.5, 60.0 * FACTOR_LAT_60)
	roughCloseDistanceChecks(t, 60.0, 0.25, 60.0, -0.25, 30.0 * FACTOR_LAT_60)
	roughCloseDistanceChecks(t, 60.0, 0.125, 60.0, -0.125, 15.0 * FACTOR_LAT_60)


	// Across equator
	roughCloseDistanceChecks(t, -1.0, -5.0, 1.0, -5.0, 120.0)
	roughCloseDistanceChecks(t, -0.5, -5.0, 0.5, -5.0, 60.0)
	roughCloseDistanceChecks(t, -0.25, -5.0, 0.25, -5.0, 30.0)
	roughCloseDistanceChecks(t, -0.125, -5.0, 0.125, -5.0, 15.0)


	roughCloseDistanceChecks(t, 45.0, 45.0, 45.1, 45.1, 7.346)
	roughCloseDistanceChecks(t, 45.1, 45.1, 45.0, 45.0, 7.346)

	roughCloseDistanceChecks(t, -65.0, -65.0, -65.1, -66.5, 38.436)
	roughCloseDistanceChecks(t, -65.1, -66.5, -65.0, -65.0, 38.436)


	// So far away that distance is guaranteed to be >=60.0
	const MAX_VALID_DIST float64 = 60.0
	roughCloseDistanceChecks(t, -65.0, -65.0, 65.1, 66.5, MAX_VALID_DIST)
	roughCloseDistanceChecks(t, 65.1, 66.5, -65.0, -65.0, MAX_VALID_DIST)
	roughCloseDistanceChecks(t, 0.0, 0.0, 0.0, 180.0, MAX_VALID_DIST)
	roughCloseDistanceChecks(t, 0.0, 90.0, 0.0, -90.0, MAX_VALID_DIST)
	roughCloseDistanceChecks(t, 90.0, 0.0, -90.0, 0.0, MAX_VALID_DIST)
	roughCloseDistanceChecks(t, 90.0, 0.0, 88.5, 180.0, MAX_VALID_DIST)
}

func TestRoughCloseDistanceLotsOfBoats(t *testing.T) {
	type boatPos struct {
		Lat float64;
		Lon float64;
	}

	const MAX_BOATS int = 2500

	boats := [MAX_BOATS]boatPos{};

	for i := 0; i < MAX_BOATS; i++ {
		boats[i].Lat = rand.Float64();
		boats[i].Lon = rand.Float64();
	}

	for i := 0; i < MAX_BOATS; i++ {
		for j := 0; j < MAX_BOATS; j++ {
			dist := roughCloseDistance(boats[i].Lat, boats[i].Lon, boats[j].Lat, boats[j].Lon)

			if (i == j) && (dist != 0.0) {
				t.Errorf("Distance between boat and itself (i == j == %d) isn't zero (dist=%f)!", i, dist)
			}
			if dist < 0.0 {
				t.Errorf("Distance (%f) is negative!", dist)
			}
		}
	}
}


func roughCloseDistanceChecks(
	t *testing.T,
	lat0 float64,
	lon0 float64,
	lat1 float64,
	lon1 float64,
	expectedDist float64) {
	const DIST_MARGIN float64 = 0.01
	const DIST_MAX_VALID float64 = 60.0

	dist := roughCloseDistance(lat0, lon0, lat1, lon1)
	distRev := roughCloseDistance(lat1, lon1, lat0, lon0)

	t.Logf("Distances from (%f,%f) to (%f,%f) are %f and %f!", lat0, lon0, lat1, lon1, dist, distRev)

	if expectedDist >= DIST_MAX_VALID {
		if dist < DIST_MAX_VALID || distRev < DIST_MAX_VALID {
			t.Errorf("Calculated distances (%f and %f) between (%f,%f) and (%f,%f) are less than expected >=%f!", dist, distRev, lat0, lon0, lat1, lon1, DIST_MAX_VALID)
		}
	} else {
		if !expectApproxDist(distRev, dist, DIST_MARGIN) {
			t.Errorf("Calculated distances (%f and %f) between (%f,%f) and (%f,%f) are too distant (>%f) from one another!", dist, distRev, lat0, lon0, lat1, lon1, DIST_MARGIN)
		}

		if !expectApproxDist(dist, expectedDist, DIST_MARGIN) {
			t.Errorf("Calculated distance (%f) between (%f,%f) and (%f,%f) is too far (>%f) from expected distance (%f)!", dist, lat0, lon0, lat1, lon1, DIST_MARGIN, expectedDist)
		}
	}
}

func expectApproxDist(dist float64, expect float64, margin float64) bool {
	return math.Abs(dist - expect) <= margin
}
