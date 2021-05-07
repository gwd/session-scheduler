package main

import (
	"flag"
	"io/ioutil"
	"log"
	"os"
	"path"
	"runtime/pprof"
	"time"

	"github.com/gwd/session-scheduler/event"
	"github.com/gwd/session-scheduler/id"
	"github.com/gwd/session-scheduler/keyvalue"
	"github.com/gwd/session-scheduler/timezones"
)

var kvs *keyvalue.KeyValueStore

const (
	ScheduleDebug        = "EventScheduleDebug"
	ScheduleDebugVerbose = "EventScheduleDebugVerbose"
	SearchAlgo           = "EventSearchAlgo"
	SearchDuration       = "EventSearchDuration"
	Validate             = "EventValidate"
	KeyDefaultLocation   = "EventDefaultLocation"
	VerificationCode     = "ServeVerificationCode"
)

var DefaultLocation = "Europe/Berlin"

var DefaultLocationTZ event.TZLocation

var TimezoneList []string

// Template and data code expect CWD to be in the same directory as
// the binary; make this so.
func cwd() {
	execpath, err := os.Executable()
	if err != nil {
		log.Printf("WARNING: Error getting executable path (%v), cannot cd to root", err)
		return
	}

	execdir := path.Dir(execpath)
	log.Printf("Changing to directory %s", execdir)

	err = os.Chdir(execdir)
	if err != nil {
		log.Printf("WARNING: Chdir to %s failed: %v", execdir, err)
		return
	}
}

func main() {
	var err error

	TimezoneList, err = timezones.GetTimezoneList()
	if err != nil {
		log.Fatal("Getting timezone list: %v", err)
	}

	cwd()

	templatesInit()

	kvs, err = keyvalue.OpenFile("data/serverconfig.sqlite")
	if err != nil {
		log.Fatal("Opening serverconfig: %v", err)
	}

	adminPwd := flag.String("admin-password", "", "Set admin password")

	flag.Var(kvs.GetFlagValue(KeyServeAddress), "address", "Address to serve http from")
	flag.Var(kvs.GetFlagValue(ScheduleDebug), "sched-debug", "Debug level for logging (default 0)")
	flag.Var(kvs.GetFlagValue(SearchAlgo), "searchalgo", "Search algorithm.  Options are heuristic, genetic, and random.")
	flag.Var(kvs.GetFlagValue(SearchDuration), "searchtime", "Duration to run search")
	flag.Var(kvs.GetFlagValue(Validate), "validate", "Extra validation of schedule consistency")
	flag.Var(kvs.GetFlagValue(KeyDefaultLocation), "default-location", "Default location to use for times")
	flag.Var(kvs.GetFlagValue(LockingMethod), "servelock", "Server locking method.  Valid options are none, quit, wait, and error")

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

	locstring, err := kvs.Get(KeyDefaultLocation)
	switch {
	case err == keyvalue.ErrNoRows:
		locstring = DefaultLocation
		if err = kvs.Set(KeyDefaultLocation, locstring); err != nil {
			log.Fatalf("Setting default location: %v", err)
		} else {
			log.Printf("Default location to %s", DefaultLocation)
		}
	case err != nil:
		log.Fatalf("Getting default location: %v", err)
	}

	DefaultLocation = locstring
	DefaultLocationTZ, err = event.LoadLocation(locstring)
	if err != nil {
		log.Fatalf("Couldn't load location %s: %v", locstring, err)
	}

	err = event.Load(event.EventOptions{AdminPwd: *adminPwd, DefaultLocation: locstring})
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
	case "editTimetable":
		EditTimetable()
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
