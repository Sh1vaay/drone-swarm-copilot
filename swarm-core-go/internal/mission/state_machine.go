package mission

import (
	"fmt"
)

// ValidateStateTransition verifies state machine handovers (BR-03)
func ValidateStateTransition(current, target string) (bool, error) {
	if current == target {
		return true, nil
	}

	switch current {
	case "Idle":
		if target == "Preflight" {
			return true, nil
		}
		return false, fmt.Errorf("Invalid transition: Idle can only go to Preflight")

	case "Preflight":
		if target == "Executing" || target == "Idle" {
			return true, nil
		}
		return false, fmt.Errorf("Invalid transition: Preflight can only go to Executing or Idle")

	case "Executing":
		if target == "Suspended" || target == "Landed" {
			return true, nil
		}
		return false, fmt.Errorf("Invalid transition: Executing cannot go directly to %s without Landing (BR-03 lock)", target)

	case "Suspended":
		if target == "Executing" || target == "Landed" {
			return true, nil
		}
		return false, fmt.Errorf("Invalid transition: Suspended can only go to Executing or Landed")

	case "Landed":
		if target == "Idle" {
			return true, nil
		}
		return false, fmt.Errorf("Invalid transition: Landed can only go to Idle")
	}

	return false, fmt.Errorf("Unknown state: %s", current)
}
