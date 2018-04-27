package main

import (
	"flag"
	"log"
	"os"
)

func main() {
	err := Event.Load()
	if err != nil {
		log.Printf("Loading schedule data: %v", err)
		os.Exit(1)
	}

	count := flag.Int("count", -1, "Number of times to iterate (tests only)")

	flag.Parse()

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
			NewTestDiscussion("")
		}
	}
}

