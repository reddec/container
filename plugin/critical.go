package plugin

import (
	"github.com/reddec/container"
	"log"
)

type critical struct {
	labels map[string]bool
	api    container.Supervisor
	logger *log.Logger
}

func (c *critical) Stopped(runnable container.Runnable, id container.ID, err error) {
	if c.labels[runnable.Label()] {
		c.logger.Println("Critical runnable stopped:", runnable.Label(), "/", id)
		go c.api.Close() // Run in separate routing to prevent dead lock
	}
}

func (c *critical) Spawned(runnable container.Runnable, id container.ID) {}

// Creates monitor that will shutdown all supervisor when critical runnable stopped
func NewCritical(api container.Supervisor, logger *log.Logger, labels ...string) container.Monitor {
	lbs := make(map[string]bool)
	for _, l := range labels {
		lbs[l] = true
	}
	return &critical{
		labels: lbs,
		logger: logger,
		api:    api,
	}
}
