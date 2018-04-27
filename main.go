package main

import (
	"log"
	"os"
)

func main() {
	err := Event.Load()
	if err != nil {
		log.Printf("Loading schedule data: %v", err)
		os.Exit(1)
	}

	serve()
}

