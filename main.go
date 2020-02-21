package main

import (
	"flag"
	"io/ioutil"
	"log"
	"os"
	"runtime/pprof"
	"time"

	"github.com/gwd/session-scheduler/event"
	"github.com/gwd/session-scheduler/id"
	"github.com/gwd/session-scheduler/keyvalue"
)

// FIXME GROSS HACK!
var Event = &event.Event

var kvs *keyvalue.KeyValueStore

const (
	ScheduleDebug        = "EventScheduleDebug"
	ScheduleDebugVerbose = "EventScheduleDebugVerbose"
	SearchAlgo           = "EventSearchAlgo"
	SearchDuration       = "EventSearchDuration"
	Validate             = "EventValidate"
	VerificationCode     = "ServeVerificationCode"
)

func main() {
	var err error

	kvs, err = keyvalue.OpenFile("data/serverconfig.db")
	if err != nil {
		log.Fatal("Opening serverconfig: %v", err)
	}

	adminPwd := flag.String("admin-password", "", "Set admin password")

	flag.Var(kvs.GetFlagValue(KeyServeAddress), "address", "Address to serve http from")
	flag.Var(kvs.GetFlagValue(ScheduleDebug), "sched-debug", "Debug level for logging (default 0)")
	flag.Var(kvs.GetFlagValue(SearchAlgo), "searchalgo", "Search algorithm.  Options are heuristic, genetic, and random.")
	flag.Var(kvs.GetFlagValue(SearchDuration), "searchtime", "Duration to run search")
	flag.Var(kvs.GetFlagValue(Validate), "validate", "Extra validation of schedule consistency")

	cpuprofile := flag.String("cpuprofile", "", "write cpu profile to `file`")

	flag.Parse()

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal("could not create CPU profile: ", err)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatal("could not start CPU profile: ", err)
		}
		defer pprof.StopCPUProfile()
	}

	{
		vcode, err := kvs.Get(VerificationCode)
		switch {
		case err == keyvalue.ErrNoRows:
			vcode = id.GenerateRawID(8)
			if err = kvs.Set(VerificationCode, vcode); err != nil {
				log.Fatalf("Setting Event Verification Code: %v", err)
			}
		case err != nil:
			log.Fatalf("Getting Event Verification Code: %v", err)
		}
	}

	err = Event.Load(event.EventOptions{AdminPwd: *adminPwd})
	if err != nil {
		log.Fatalf("Loading schedule data: %v", err)
	}

	cmd := flag.Arg(0)
	if cmd == "" {
		cmd = "serve"
	}

	switch cmd {
	case "serve":
		serve()
	case "schedule":
		MakeSchedule(false)
	default:
		log.Fatalf("Unknown command: %s", cmd)
	}

}

func getSearchDuration() time.Duration {
	durationString, err := kvs.Get(SearchDuration)
	var duration time.Duration
	if err != nil {
		duration, err = time.ParseDuration(durationString)
	}

	if err != nil {
		// Default search time 5 seconds
		return time.Second * 5
	}
	return duration
}

func MakeSchedule(async bool) error {
	opt := event.SearchOptions{Async: async}

	algostring, err := kvs.Get(SearchAlgo)
	switch {
	case err == keyvalue.ErrNoRows:
		opt.Algo = event.SearchRandom
	case err != nil:
		log.Fatalf("Error getting keyvalue: %v", err)
	default:
		opt.Algo = event.SearchAlgo(algostring)
	}

	opt.Validate = kvs.GetBoolDef(Validate)

	if kvs.GetBoolDef(ScheduleDebug) {
		opt.DebugLevel = 1
		opt.Debug = log.New(os.Stderr, "schedule.go ", log.LstdFlags)
		if kvs.GetBoolDef(ScheduleDebugVerbose) {
			opt.DebugLevel = 2
		}
	} else {
		opt.Debug = log.New(ioutil.Discard, "schedule.go ", log.LstdFlags)
	}

	opt.SearchDuration = getSearchDuration()

	return event.MakeSchedule(opt)
}
