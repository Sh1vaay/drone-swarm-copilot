package formation

import (
	"sort"

	"swarm-core-go/internal/core"
)

// CalculateProportionalWidth segments a search area proportionally by battery charge (BR-02).
func CalculateProportionalWidth(droneID string, batteries map[string]int32, totalWidth float64) float64 {
	if len(batteries) == 0 {
		return 0
	}

	var sumBatteries int32
	for _, b := range batteries {
		sumBatteries += b
	}
	if sumBatteries == 0 {
		return 0
	}

	droneBattery, ok := batteries[droneID]
	if !ok {
		return 0
	}
	return totalWidth * (float64(droneBattery) / float64(sumBatteries))
}

// AllocateMissionTasks dynamically distributes tasks based on real-time hardware capabilities.
func AllocateMissionTasks(intent string, squadDrones []core.MockDroneState) map[string]string {
	allocations := make(map[string]string)
	if len(squadDrones) == 0 {
		return allocations
	}

	for _, d := range squadDrones {
		allocations[d.DroneID] = "TRACK"
	}

	if intent != "SEARCH" && intent != "SEARCH_AREA" && intent != "RECON" {
		return allocations
	}

	total := len(squadDrones)

	sortedByComm := append([]core.MockDroneState(nil), squadDrones...)
	sort.Slice(sortedByComm, func(i, j int) bool {
		return sortedByComm[i].CommQuality > sortedByComm[j].CommQuality
	})

	relayCount := total / 10
	if relayCount == 0 && total >= 3 {
		relayCount = 1
	}
	for i := 0; i < relayCount && i < len(sortedByComm); i++ {
		allocations[sortedByComm[i].DroneID] = "RELAY"
	}

	var remaining []core.MockDroneState
	for _, d := range squadDrones {
		if allocations[d.DroneID] != "RELAY" {
			remaining = append(remaining, d)
		}
	}
	sort.Slice(remaining, func(i, j int) bool {
		return remaining[i].TrustScore > remaining[j].TrustScore
	})

	mapCount := len(remaining) / 5
	if mapCount == 0 && len(remaining) >= 2 {
		mapCount = 1
	}
	for i := 0; i < mapCount && i < len(remaining); i++ {
		allocations[remaining[i].DroneID] = "MAPPING"
	}

	var remainingForSearch []core.MockDroneState
	for _, d := range remaining {
		if allocations[d.DroneID] != "MAPPING" {
			remainingForSearch = append(remainingForSearch, d)
		}
	}
	sort.Slice(remainingForSearch, func(i, j int) bool {
		return remainingForSearch[i].Battery > remainingForSearch[j].Battery
	})

	searchCount := int(float64(total) * 0.6)
	if searchCount > len(remainingForSearch) {
		searchCount = len(remainingForSearch)
	}
	for i := 0; i < searchCount; i++ {
		allocations[remainingForSearch[i].DroneID] = "SEARCH"
	}

	return allocations
}
