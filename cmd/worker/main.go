package main

import (
	"log"

	"github.com/nevermore222/sangyu-record/internal/config"
)

func main() {
	if _, err := config.Load(); err != nil {
		log.Fatal(err)
	}
	log.Print("worker bootstrap complete; task handlers are added in Task 5")
}
