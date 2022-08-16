package main

import (
	"github.com/vladopajic/go-actor/actor"
)

// This program will demonstrate how to create actors for
// producer-consumer use case, where
// producer will create incremented number on every 1 second interval and
// consumer will print whaterver number it receives
func main() {
	numC := make(chan int)

	// Producer and consumer workers are created with same channel
	// so that producer worker can write directly to consumer worker
	pw := &producerWorker{outC: numC}
	cw1 := &consumerWorker{inC: numC, id: 1}
	cw2 := &consumerWorker{inC: numC, id: 2}

	// Create actors using these workers
	a := actor.Combine(
		actor.New(pw),

		// Note: We don't need two consumer actors, but we create them anyway
		// for the sake of demonstration since having one or more consumers
		// will produce the same result. Message on stdout will be written by
		// first consumer that reads from numC channel.
		actor.New(cw1),
		actor.New(cw2),
	)

	// Finally we start all actors at once
	a.Start()
	defer a.Stop()

	select {}
}