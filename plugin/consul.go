package plugin

import (
	"github.com/reddec/container"
	"time"

	"fmt"
	"os"

	"github.com/hashicorp/consul/api"
	"log"
	"io"
	"sync"
)

type Consul interface {
	io.Closer
	container.Monitor
}

type consul struct {
	RegisterLabels            map[string]ConsulRegistration
	AutoDeregistrationTimeout time.Duration
	TTL                       time.Duration
	Log                       *log.Logger
	Client                    *api.Client
	matched                   map[string]struct{}
	lock                      sync.Mutex
	stop                      chan struct{}
}

func (c *consul) Spawned(runnable container.Runnable, id container.ID) {
	info, exists := c.RegisterLabels[runnable.Label()]
	if !exists {
		return
	}
	dereg := c.AutoDeregistrationTimeout
	if dereg < c.TTL {
		dereg = 2 * c.TTL
	}
	if dereg < 1*time.Minute {
		dereg = 1 * time.Minute
	}
	err := c.Client.Agent().ServiceRegister(&api.AgentServiceRegistration{
		Name: runnable.Label(),
		Tags: []string{fmt.Sprintf("%v", os.Getpid())},
		Check: &api.AgentServiceCheck{
			DeregisterCriticalServiceAfter: dereg.String(),
		},
	})
	if err != nil {
		c.Log.Println("Can't register service", id, "in Consul:", err)
	} else {
		checkID := runnable.Label() + ":ttl"
		reg := api.AgentCheckRegistration{}
		reg.Name = checkID
		reg.TTL = c.TTL.String()
		reg.ServiceID = runnable.Label()

		if !info.Permanent {
			reg.DeregisterCriticalServiceAfter = dereg.String()
		}

		err = c.Client.Agent().CheckRegister(&reg)
		if err != nil {
			c.Log.Println("Can't register service TTL check", id, "in Consul:", err)
		} else {
			c.Log.Println("Service", runnable.Label(), "registered in Consul")
			c.lock.Lock()
			c.matched[checkID] = struct{}{}
			c.lock.Unlock()
		}
	}
}

func (c *consul) checkLoop() {
LOOP:
	for {
		select {
		case <-time.After(c.TTL / 2):
			c.updateChecks()
		case <-c.stop:
			break LOOP
		}
	}
}

func (c *consul) updateChecks() {
	c.lock.Lock()
	wg := sync.WaitGroup{}
	wg.Add(len(c.matched))
	for id, _ := range c.matched {
		go func(id string) {
			defer wg.Done()
			err := c.Client.Agent().UpdateTTL(id, "application running", "pass")
			if err != nil {
				c.Log.Println("Can't update TTL for service", id, "in Consul:", err)
			}
		}(id)
	}
	c.lock.Unlock()
	wg.Wait()
}

func (c *consul) Close() error {
	close(c.stop)
	return nil
}

func (c *consul) Stopped(runnable container.Runnable, id container.ID, err error) {
	info, exists := c.RegisterLabels[runnable.Label()]
	if !exists {
		return
	}
	c.lock.Lock()
	delete(c.matched, runnable.Label()+":ttl")
	c.lock.Unlock()

	if !info.Permanent {
		err = c.Client.Agent().ServiceDeregister(runnable.Label())
		if err != nil {
			c.Log.Println("Can't deregister service", runnable.Label(), "in Consul:", err)
		} else {
			c.Log.Println("Service", runnable.Label(), "deregistered in Consul")
		}
	}
}

// Define options for consul registration
type ConsulRegistration struct {
	// Keep service registered even if stopped
	Permanent bool   `json:"permanent,omitempty" yaml:"permanent,omitempty" ini:"permanent,omitempty"`
	// Name of service
	Label string `json:"label" yaml:"label" ini:"label"`
}

// Creates new Consul agent that will register and deregister specified services
//
func NewConsul(client *api.Client, TTL time.Duration, deReg time.Duration, logger *log.Logger, labels []ConsulRegistration) Consul {
	lbs := make(map[string]ConsulRegistration)
	for _, v := range labels {
		lbs[v.Label] = v
	}
	cons := &consul{
		RegisterLabels:            lbs,
		AutoDeregistrationTimeout: deReg,
		TTL:                       TTL,
		Log:                       logger,
		Client:                    client,
		stop:                      make(chan struct{}, 1),
		matched:                   make(map[string]struct{}),
	}
	go cons.checkLoop()
	return cons
}
