package event

import (
	"errors"
	"log"
	"time"
	//"github.com/hako/durafmt"
)

type SearchAlgo string

const (
	SearchHeuristicOnly = SearchAlgo("heuristic")
	//SearchGenetic       = SearchAlgo("genetic")
	SearchRandom = SearchAlgo("random")
)

type SearchOptions struct {
	Async          bool
	Algo           SearchAlgo
	Validate       bool
	DebugLevel     int
	SearchDuration time.Duration
	Debug          *log.Logger
}

var opt SearchOptions

func MakeSchedule(optArg SearchOptions) error {
	// FIXME: Schedule

	return errors.New("Not Implemented!")
}

func SchedLastUpdate() string {
	lastUpdate := "Never"
	// if event.ScheduleV2 != nil {
	// 	lastUpdate = durafmt.ParseShort(time.Since(event.ScheduleV2.Created)).String() + " ago"
	// }
	return lastUpdate
}

type SchedState int

const (
	SchedStateCurrent = SchedState(iota)
	SchedStateModified
	SchedStateRunning
)

func SchedGetState() SchedState {
	return SchedStateCurrent
}
