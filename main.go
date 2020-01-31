package main

import (
	"flag"
	"log"
	"os"
	"runtime/pprof"

	disc "github.com/gwd/session-scheduler/discussions"
)

// FIXME GROSS HACK!
var Event = &disc.Event

func main() {
	count := flag.Int("count", -1, "Number of times to iterate (tests only)")
	flag.StringVar(&disc.OptAdminPassword, "admin-password", "", "Set admin password")
	flag.StringVar(&OptServeAddress, "address", OptServeAddress, "Address to serve http from")
	flag.BoolVar(&disc.OptSchedDebug, "sched-debug", false, "Enanable scheduler debug logging")
	flag.StringVar(&OptSearchAlgo, "searchalgo", string(disc.SearchRandom), "Search algorithm.  Options are heuristic, genetic, and random.")
	flag.StringVar(&disc.OptSearchDurationString, "searchtime", "60s", "Duration to run search")
	flag.BoolVar(&disc.OptValidate, "validate", false, "Extra validation of schedule consistency")

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

	disc.ScheduleInit()

	err := Event.Load()
	if err != nil {
		log.Fatalf("Loading schedule data: %v", err)
	}

	cmd := flag.Arg(0)
	if cmd == "" {
		cmd = "serve"
	}

	if cmd == "serve" {
		if *count != -1 {
			log.Fatal("Cannot use -count with serve")
		}
	} else {
		if *count == -1 {
			*count = 0
		}
	}

	switch cmd {
	case "serve":
		serve()
	case "schedule":
		disc.MakeSchedule(disc.SearchAlgo(OptSearchAlgo), false)
	default:
		log.Fatalf("Unknown command: %s", cmd)
	}

}
