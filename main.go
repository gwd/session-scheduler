package main

import (
	"flag"
	"log"
	"os"
	"runtime/pprof"
)

func main() {
	count := flag.Int("count", -1, "Number of times to iterate (tests only)")
	flag.StringVar(&OptAdminPassword, "admin-password", "", "Set admin password")
	flag.StringVar(&OptServeAddress, "address", OptServeAddress, "Address to serve http from")
	flag.BoolVar(&OptSchedDebug, "sched-debug", false, "Enanable scheduler debug logging")
	flag.StringVar(&OptSearchAlgo, "searchalgo", string(SearchGenetic), "Search algorithm.  Options are heuristic, genetic, and random.")
	flag.StringVar(&OptSearchDurationString, "searchtime", "30s", "Duration to run search")
	flag.BoolVar(&OptCrossover, "crossover", true, "Enable the crossover function")

	
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

	ScheduleInit()
	
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
	case "testuser":
		for ; *count > 0 ; *count-- {
			NewTestUser()
		}
	case "testdisc":
		for ; *count > 0 ; *count-- {
			NewTestDiscussion(nil)
		}
	case "testpopulate":
		if *count != -500 {
			log.Fatal("WARNING: populate will erase the current database.  If you really want to do this, pass a count value of -500.")
		}
		TestPopulate()
	case "testinterest":
		TestGenerateInterest()
	case "schedule":
		MakeSchedule()
	default:
		log.Fatalf("Unknown command: %s", cmd)
	}
	
}

