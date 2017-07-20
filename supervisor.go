package container

import (
	"context"
	"time"
)

// ID of runnable
type ID string

// Base instance that can be run.
type Runnable interface {
	Label() string
	Run(ctx context.Context) error
}

// Factory that creates runnable
type Factory func() (Runnable, error)

// General information about runnable
type RunnableInfo struct {
	Instance Runnable
	ID       ID
}

// Monitor events of state in supervisor
type Monitor interface {
	Spawned(runnable Runnable, id ID)
	Stopped(runnable Runnable, id ID, err error)
}

// Event when runnable started
type WatchEventStarted RunnableInfo

// Event when runnable stopped
type WatchEventStopped struct {
	RunnableInfo
	Error error
}

// Stream of events (WatchEventStarted or WatchEventStopped) while process running
type WatchEvents <-chan interface{}

// Supervisor monitors group of processes
type Supervisor interface {
	// Watch runnable and restart if needed but not more then restartLimit. If restartLimit is negative, it's mean infinity.
	// If runnable stopped with error and stopOnError is true, then watch loop exits.
	// Events MUST be consumed
	Watch(ctx context.Context, factory Factory, restartLimit int, restartDelay time.Duration, stopOnError bool) WatchEvents
	// Spawn and monitor one runnable in background. Returns generated ID, done channel (buffered) and stop function
	Spawn(runnable Runnable) (ID, <-chan error, func())
	// SpawnFunc creates and spawn ClosureWrapper runnable
	SpawnFunc(label string, closure func(ctx context.Context) error) (ID, <-chan error, func())
	// List all runnables
	List() []RunnableInfo
	// Get runnable by generated ID
	Get(ID) Runnable
	// Events emitter of runnable
	Events() MonitorEvents
	// Close supervisor and stops all processes
	Close()
}

// Wait for finish
func Wait(events WatchEvents) error {
	var err error
	for event := range events {
		switch v := event.(type) {
		case WatchEventStopped:
			err = v.Error
		}
	}
	return err
}

// Create factory that repeats closure
func RepeatFunc(label string, fn func(ctx context.Context) error) Factory {
	return func() (Runnable, error) {
		return &closure{instance: fn, label: label}, nil
	}
}

//go:generate gbus Monitor supervisor.go
