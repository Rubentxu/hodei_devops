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
	Skipped
	Done
)

func (s TaskState) String() []string {
	return []string{"Pending", "Scheduled", "Running", "Completed", "Failed", "Stopped", "Unknown"}
}

func (s TaskState) IsTerminal() bool {
	return s == Done || s == Failed
}

var stateTransitionMap = map[TaskState][]TaskState{
	Pending:   {Scheduled},
	Scheduled: {Scheduled, Running, Failed, Stopped},
	Running:   {Running, Completed, Failed, Scheduled, Stopped},
	Completed: {Done},
	Failed:    {Scheduled},
	Stopped:   {Scheduled},
	Skipped:   {Done},
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

func AllTaskStates() []TaskState {
	return []TaskState{
		Pending,
		Scheduled,
		Running,
		Completed,
		Failed,
		Stopped,
		Skipped,
		Unknown,
		Done,
	}
}
