package container

import (
	"context"
	"github.com/satori/go.uuid"
	"time"
	"sync"
	"log"
)

type supervisor struct {
	sync.Mutex
	closed bool
	global context.Context
	stop   func()
	childs map[ID]Runnable
	events MonitorEventEmitter
	group  sync.WaitGroup
	logger *log.Logger
}

func NewSupervisor(logger *log.Logger) Supervisor {
	ctx, stp := context.WithCancel(context.Background())
	return &supervisor{
		global: ctx,
		stop:   stp,
		logger: logger,
	}
}

func (su *supervisor) Events() MonitorEvents { return &su.events }

func (su *supervisor) register(runnable Runnable, id ID) {
	su.Lock()
	defer su.Unlock()

	if su.childs == nil {
		su.childs = make(map[ID]Runnable)
	}
	su.childs[id] = runnable

	su.logger.Println("Runnable", runnable.Label(), "registered with id", id)
}

func (su *supervisor) deregister(id ID) {
	su.Lock()
	defer su.Unlock()
	if su.childs == nil {
		return
	}
	delete(su.childs, id)
	su.logger.Println("Runnable with id", id, "removed")
}

func childMonitor(local context.Context, runnable Runnable) error {
	end := make(chan error, 1)
	go func() {
		end <- runnable.Run(local)
	}()
	select {
	case err := <-end:
		return err
	}
}

func (su *supervisor) Spawn(runnable Runnable) (ID, <-chan error, func()) {
	if su.closed {
		return "", nil, nil
	}
	id := ID(uuid.NewV4().String())
	su.register(runnable, id)
	su.group.Add(1)
	end := make(chan error, 1)
	ctx, stp := context.WithCancel(su.global)
	go func() {
		defer su.group.Done()
		defer su.deregister(id)
		su.events.Spawned(runnable, id)
		err := childMonitor(ctx, runnable)
		su.events.Stopped(runnable, id, err)
		end <- err
		su.logger.Println("Runnable", runnable.Label(), "with id", id, "stopped")
	}()
	su.logger.Println("Runnable", runnable.Label(), "spawned with id", id)
	return id, end, stp
}

func (su *supervisor) SpawnFunc(label string, fn func(ctx context.Context) error) (ID, <-chan error, func()) {
	return su.Spawn(&closure{label: label, instance: fn})
}

func (su *supervisor) Get(id ID) Runnable {
	if su.childs == nil {
		return nil
	}
	su.Lock()
	defer su.Unlock()
	if su.childs == nil {
		return nil
	}
	return su.childs[id]
}

func (su *supervisor) List() []RunnableInfo {
	var info []RunnableInfo
	su.Lock()
	defer su.Unlock()
	for id, run := range su.childs {
		info = append(info, RunnableInfo{ID: id, Instance: run})
	}
	return info
}

func (su *supervisor) Watch(ctx context.Context, factory Factory, restartLimit int, restartDelay time.Duration, stopOnError bool) WatchEvents {
	events := make(chan interface{}, 1)
	go func() {
		defer close(events)
	LOOP:
		for restartLimit != 0 {
			run, err := factory()
			if err == nil {
				id, done, stop := su.Spawn(run)
				if stop == nil {
					// supervisor closed
					su.logger.Println("Can't respawn", run.Label(), "because supervisor is closed")
					break LOOP
				}
				info := RunnableInfo{Instance: run, ID: id}
				events <- WatchEventStarted(info)

				select {
				case err := <-done:
					events <- WatchEventStopped{RunnableInfo: info, Error: err}
					if err != nil && stopOnError {
						su.logger.Println("Monitoring of runnable", run.Label(), "with id", id, "stopped due to", err, "and flag enabled stopOnError")
						break LOOP
					}
				case <-ctx.Done():
					stop()
					err := <-done
					su.logger.Println("Monitoring of runnable", run.Label(), "with id", id, "interrupted manually. Finished with result:", err)
					events <- WatchEventStopped{RunnableInfo: info, Error: err}
					break LOOP
				case <-su.global.Done():
					stop()
					err := <-done
					su.logger.Println("Monitoring of runnable", run.Label(), "with id", id, "interrupted (supervisor closed) manually. Finished with result:", err)
					events <- WatchEventStopped{RunnableInfo: info, Error: err}
					break LOOP
				}
			}
			if restartLimit == 0 {
				break
			}
			su.logger.Println("Monitor will respawn", run.Label(), "after", restartDelay)
			select {
			case <-time.After(restartDelay):

			case <-ctx.Done():
				break LOOP
			case <-su.global.Done():
				break LOOP
			}
			restartLimit--
		}
	}()

	return events
}

func (su *supervisor) Close() {
	if su.closed {
		return
	}
	su.Lock()
	if su.closed {
		su.Unlock()
		return
	}
	su.logger.Println("Closing supervisor")
	su.closed = true
	su.Unlock()
	su.stop()
	su.group.Wait()
	su.logger.Println("Supervisor closed")
}

type closure struct {
	instance func(ctx context.Context) error
	label    string
}

func (cl *closure) Run(ctx context.Context) error {
	return cl.instance(ctx)
}

func (cl *closure) Label() string { return cl.label }
