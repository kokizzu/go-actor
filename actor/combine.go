package actor

import (
	"sync"
	"sync/atomic"
)

// Combine returns a builder that creates a single Actor by combining multiple
// specified Actors into one.
//
// This function allows you to aggregate multiple Actor instances, enabling
// collective management of their lifecycles. When the Start or Stop methods
// are called on the combined Actor, the respective method will be invoked on
// all underlying Actors in the order they were provided.
func Combine(actors ...Actor) *CombineBuilder {
	return &CombineBuilder{
		actors: actors,
	}
}

type CombineBuilder struct {
	actors  []Actor
	options options
}

// Build returns the combined Actor created by the CombineBuilder.
//
// This method finalizes the configuration and returns a new Actor instance
// that integrates all the Actors specified during the building process.
// The returned Actor will manage the lifecycle of each underlying Actor,
// allowing you to start or stop them collectively.
func (b *CombineBuilder) Build() Actor {
	if len(b.actors) == 0 {
		options := combinedOptionsToRegularList(b.options.Combined)
		return Idle(options...)
	}

	a := &combinedActor{
		actors:   b.actors,
		options:  b.options.Combined,
		stopping: &atomic.Bool{},
	}

	a.actors = wrapActors(a.actors, a.onActorStopped)

	return a
}

// WithOptions adds configuration options for the combined Actor.
//
// This method allows you to specify one or more CombinedOption settings
// that customize the behavior of the combined Actor. The provided options
// will be applied to the Actor when it is built, allowing for fine-tuning
// of its lifecycle management and behavior.
func (b *CombineBuilder) WithOptions(opt ...CombinedOption) *CombineBuilder {
	b.options = newOptions(opt)
	return b
}

type combinedActor struct {
	actors  []Actor
	options optionsCombined

	ctx          *context
	runningCount atomic.Int64
	running      bool
	runningLock  sync.Mutex
	stopping     *atomic.Bool
}

func (a *combinedActor) onActorStopped() {
	a.runningLock.Lock()

	runningCount := a.runningCount.Add(-1)
	wasRunning := a.running
	a.running = runningCount != 0

	a.runningLock.Unlock()

	// Last actor to end should call onStopFunc
	if runningCount == 0 && wasRunning && a.options.OnStopFunc != nil {
		a.options.OnStopFunc()
	}

	// First actor to stop should stop other actors
	if a.options.StopTogether && !a.stopping.Load() {
		// Run stop in goroutine because wrapped actor
		// should not wait for other actors to stop.
		//
		// Also if a.Stop() is called from same goroutine it would
		// be recursive call without exit condition. Therefore
		// it is need to call a.Stop() from other goroutine,
		// regardless of first invariant.
		go a.Stop()
	}
}

func (a *combinedActor) Stop() {
	a.runningLock.Lock()

	if !a.running || a.stopping.Swap(true) {
		a.runningLock.Unlock()
		return
	}

	a.ctx.end()

	a.runningLock.Unlock()

	if a.options.StopParallel {
		stopAllParallel(a.actors)
	} else {
		stopAll(a.actors)
	}
}

func (a *combinedActor) Start() {
	a.runningLock.Lock()

	if a.running {
		a.runningLock.Unlock()
		return
	}

	ctx := newContext()
	a.ctx = ctx
	a.stopping.Store(false)
	a.running = true
	a.runningCount.Add(int64(len(a.actors)))

	a.runningLock.Unlock()

	if fn := a.options.OnStartFunc; fn != nil {
		fn(ctx)
	}

	startAll(a.actors)
}

func startAll(actors []Actor) {
	for _, a := range actors {
		a.Start()
	}
}

func stopAll(actors []Actor) {
	for _, a := range actors {
		a.Stop()
	}
}

func stopAllParallel(actors []Actor) {
	wg := sync.WaitGroup{}
	wg.Add(len(actors))
	defer wg.Wait()

	for _, a := range actors {
		go func() {
			a.Stop()
			wg.Done()
		}()
	}
}

func wrapActors(
	actors []Actor,
	onStopFunc func(),
) []Actor {
	wrapActorStruct := func(a *actor) *actor {
		prevOnStopFunc := a.options.OnStopFunc

		a.options.OnStopFunc = func() {
			if prevOnStopFunc != nil {
				prevOnStopFunc()
			}

			onStopFunc()
		}

		return a
	}

	wrapCombinedActorStruct := func(a *combinedActor) *combinedActor {
		prevOnStopFunc := a.options.OnStopFunc

		a.options.OnStopFunc = func() {
			if prevOnStopFunc != nil {
				prevOnStopFunc()
			}

			onStopFunc()
		}

		return a
	}

	wrapActorInterface := func(a Actor) Actor {
		return &wrappedActor{
			actor:      a,
			onStopFunc: onStopFunc,
		}
	}

	for i, a := range actors {
		switch v := a.(type) {
		case *actor:
			actors[i] = wrapActorStruct(v)
		case *combinedActor:
			actors[i] = wrapCombinedActorStruct(v)
		default:
			actors[i] = wrapActorInterface(v)
		}
	}

	return actors
}

type wrappedActor struct {
	actor      Actor
	onStopFunc func()
}

func (a *wrappedActor) Start() {
	a.actor.Start()
}

func (a *wrappedActor) Stop() {
	a.actor.Stop()
	a.onStopFunc()
}

func combinedOptionsToRegularList(combined optionsCombined) []Option {
	var options []Option

	if fn := combined.OnStartFunc; fn != nil {
		options = append(options, OptOnStart(fn))
	}

	if fn := combined.OnStopFunc; fn != nil {
		options = append(options, OptOnStop(fn))
	}

	return options
}
