package main

import (
	"flag"
	"log"
	"os"
	"runtime/pprof"

	"github.com/gwd/session-scheduler/event"
	"github.com/gwd/session-scheduler/keyvalue"
)

// FIXME GROSS HACK!
var Event = &event.Event

var kvs *keyvalue.KeyValueStore

func main() {
	var err error

	kvs, err = keyvalue.OpenFile("data/serverconfig.db")
	if err != nil {
		log.Fatal("Opening serverconfig: %v", err)
	}

	adminPwd := flag.String("admin-password", "", "Set admin password")

	flag.Var(kvs.GetFlagValue(KeyServeAddress), "address", "Address to serve http from")
	flag.Var(kvs.GetFlagValue(event.EventScheduleDebug), "sched-debug", "Enanable scheduler debug logging")
	flag.Var(kvs.GetFlagValue(event.EventSearchAlgo), "searchalgo", "Search algorithm.  Options are heuristic, genetic, and random.")
	flag.Var(kvs.GetFlagValue(event.EventSearchDuration), "searchtime", "Duration to run search")
	flag.Var(kvs.GetFlagValue(event.EventValidate), "validate", "Extra validation of schedule consistency")

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

	err = Event.Load(event.EventOptions{KeyValueStore: kvs, AdminPwd: *adminPwd})
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
		algostring, err := kvs.Get("EventSearchAlgo")
		algo := event.SearchAlgo(algostring)
		if err == keyvalue.ErrNoRows {
			algo = event.SearchRandom
		} else if err != nil {
			log.Fatalf("Error getting keyvalue: %v", err)
		}
		event.MakeSchedule(event.SearchAlgo(algo), false)
	default:
		log.Fatalf("Unknown command: %s", cmd)
	}

}
