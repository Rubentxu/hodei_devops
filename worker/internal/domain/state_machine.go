package domain

import "log"

type State int

const (
	Pending State = iota
	Scheduled
	Running
	Completed
	Failed
	Stopped
	Unknown
)

func (s State) String() []string {
	return []string{"Pending", "Scheduled", "Running", "Completed", "Failed", "Stopped", "Unknown"}
}

var stateTransitionMap = map[State][]State{
	Pending:   []State{Scheduled},
	Scheduled: []State{Scheduled, Running, Failed, Stopped},
	Running:   []State{Running, Completed, Failed, Scheduled, Stopped},
	Completed: []State{},
	Failed:    []State{Scheduled},
	Stopped:   []State{Scheduled},
	Unknown:   []State{Pending, Scheduled, Running, Completed, Failed, Stopped},
}

func Contains(states []State, state State) bool {
	for _, s := range states {
		if s == state {
			return true
		}
	}
	return false
}

func ValidStateTransition(src State, dst State) bool {
	log.Printf("attempting to transition from %#v to %#v\n", src, dst)
	return Contains(stateTransitionMap[src], dst)
}
