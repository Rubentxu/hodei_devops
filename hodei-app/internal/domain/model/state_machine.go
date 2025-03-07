package model

import "log"

type TaskState int

const (
	Pending TaskState = iota
	Scheduled
	Running
	Completed
	Failed
	Stopped
	Unknown
	Done
)

func (s TaskState) String() []string {
	return []string{"Pending", "Scheduled", "Running", "Completed", "Failed", "Stopped", "Unknown"}
}

var stateTransitionMap = map[TaskState][]TaskState{
	Pending:   {Scheduled},
	Scheduled: {Scheduled, Running, Failed, Stopped},
	Running:   {Running, Completed, Failed, Scheduled, Stopped},
	Completed: {Done},
	Failed:    {Scheduled},
	Stopped:   {Scheduled},
	Unknown:   {Pending, Scheduled, Running, Completed, Failed, Stopped},
}

func Contains(states []TaskState, state TaskState) bool {
	for _, s := range states {
		if s == state {
			return true
		}
	}
	return false
}

func ValidStateTransition(src TaskState, dst TaskState) bool {
	log.Printf("attempting to transition from %#v to %#v\n", src, dst)
	return Contains(stateTransitionMap[src], dst)
}
