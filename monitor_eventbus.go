package container

import (
	"sync"
	"reflect"
)

// How to use it:
//
// type Sample struct {
//      emitter MonitorEventEmitter
// }
//
// func (s *Sample) Events() MonitorEvents { return &s.emitter }
//
// ...
//
// func (s *Sample) SomeJob() {
//    ...
//    s.Spawned(runnable, id)  // emit event Spawned(runnable Runnable, id ID)
//    ...
// }
//

type (
	MonitorSpawnedHandlerFunc func(runnable Runnable, id ID)            // Listener handler function for event 'Spawned'
	MonitorStoppedHandlerFunc func(runnable Runnable, id ID, err error) // Listener handler function for event 'Stopped'
)

// MonitorEventEmitter implements events listener and events emitter operations
// for events Spawned, Stopped
type MonitorEventEmitter struct {
	MonitorEvents // Implements listener operations
	lockSpawned sync.RWMutex
	onSpawned   []MonitorSpawnedHandlerFunc
	lockStopped sync.RWMutex
	onStopped   []MonitorStoppedHandlerFunc
}

// MonitorEvents is a client side of event bus that allows subscribe to
// Spawned, Stopped events
type MonitorEvents interface {
	// Spawned adds event listener for event 'Spawned'
	OnSpawned(handler MonitorSpawnedHandlerFunc)

	// NoSpawned excludes event listener
	NoSpawned(handler MonitorSpawnedHandlerFunc)

	// Stopped adds event listener for event 'Stopped'
	OnStopped(handler MonitorStoppedHandlerFunc)

	// NoStopped excludes event listener
	NoStopped(handler MonitorStoppedHandlerFunc)

	// AddHandler adds handler for events (Spawned, Stopped)
	AddHandler(handler Monitor)

	// RemoveHandler remove handler for events
	RemoveHandler(handler Monitor)
}

// OnSpawned adds event listener for event 'Spawned'
func (bus *MonitorEventEmitter) OnSpawned(handler MonitorSpawnedHandlerFunc) {
	bus.lockSpawned.Lock()
	defer bus.lockSpawned.Unlock()
	bus.onSpawned = append(bus.onSpawned, handler)
}

// NoSpawned excludes event listener
func (bus *MonitorEventEmitter) NoSpawned(handler MonitorSpawnedHandlerFunc) {
	bus.lockSpawned.Lock()
	defer bus.lockSpawned.Unlock()
	var res []MonitorSpawnedHandlerFunc
	refVal := reflect.ValueOf(handler).Pointer()
	for _, f := range bus.onSpawned {
		if reflect.ValueOf(f).Pointer() != refVal {
			res = append(res, f)
		}
	}
	bus.onSpawned = res
}

// Spawned emits event with same name
func (bus *MonitorEventEmitter) Spawned(runnable Runnable, id ID) {
	bus.lockSpawned.RLock()
	defer bus.lockSpawned.RUnlock()
	for _, f := range bus.onSpawned {
		f(runnable, id)
	}
}

// OnStopped adds event listener for event 'Stopped'
func (bus *MonitorEventEmitter) OnStopped(handler MonitorStoppedHandlerFunc) {
	bus.lockStopped.Lock()
	defer bus.lockStopped.Unlock()
	bus.onStopped = append(bus.onStopped, handler)
}

// NoStopped excludes event listener
func (bus *MonitorEventEmitter) NoStopped(handler MonitorStoppedHandlerFunc) {
	bus.lockStopped.Lock()
	defer bus.lockStopped.Unlock()
	var res []MonitorStoppedHandlerFunc
	refVal := reflect.ValueOf(handler).Pointer()
	for _, f := range bus.onStopped {
		if reflect.ValueOf(f).Pointer() != refVal {
			res = append(res, f)
		}
	}
	bus.onStopped = res
}

// Stopped emits event with same name
func (bus *MonitorEventEmitter) Stopped(runnable Runnable, id ID, err error) {
	bus.lockStopped.RLock()
	defer bus.lockStopped.RUnlock()
	for _, f := range bus.onStopped {
		f(runnable, id, err)
	}
}

// AddHandler adds handler for events (Spawned, Stopped)
func (bus *MonitorEventEmitter) AddHandler(handler Monitor) {
	bus.OnSpawned(handler.Spawned)
	bus.OnStopped(handler.Stopped)
}

// RemoveHandler remove handler for events
func (bus *MonitorEventEmitter) RemoveHandler(handler Monitor) {
	bus.NoSpawned(handler.Spawned)
	bus.NoStopped(handler.Stopped)
}
