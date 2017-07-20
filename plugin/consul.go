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
	matched                   []string
	lock                      sync.Mutex
	stop                      chan struct{}
}

func (c *consul) Spawned(runnable container.Runnable, id container.ID) {
	_, exists := c.RegisterLabels[runnable.Label()]
	if !exists {
		return
	}
	err := c.Client.Agent().ServiceRegister(&api.AgentServiceRegistration{
		ID:   string(id),
		Name: runnable.Label(),
		Tags: []string{fmt.Sprintf("%v", os.Getpid())},
		Check: &api.AgentServiceCheck{
			DeregisterCriticalServiceAfter: c.AutoDeregistrationTimeout.String(),
		},
	})
	if err != nil {
		c.Log.Println("Can't register service", id, "in Consul:", err)
	} else {
		reg := api.AgentCheckRegistration{}
		reg.Name = string(id) + ":ttl"
		reg.ID = string(id)
		reg.TTL = c.TTL.String()
		if c.AutoDeregistrationTimeout != 0 {
			reg.DeregisterCriticalServiceAfter = c.AutoDeregistrationTimeout.String()
		}
		reg.ServiceID = runnable.Label()
		err = c.Client.Agent().CheckRegister(&reg)
		if err != nil {
			c.Log.Println("Can't register service TTL check", id, "in Consul:", err)
		} else {
			c.Log.Println("Service", runnable.Label(), "registered in Consul")
			c.lock.Lock()
			c.matched = append(c.matched, string(id))
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
	var clone []string
	c.lock.Lock()
	clone = append(clone, c.matched...)
	c.lock.Unlock()
	wg := sync.WaitGroup{}
	wg.Add(len(clone))
	for _, id := range clone {
		go func(id string) {
			defer wg.Done()
			err := c.Client.Agent().UpdateTTL(id, "application running", "pass")
			if err != nil {
				c.Log.Println("Can't update TTL for service", id, "in Consul:", err)
			}
		}(id)
	}
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
	err = c.Client.Agent().CheckDeregister(string(id))
	if err != nil {
		c.Log.Println("Can't deregister service TTL check", id, "in Consul:", err)
	} else {
		c.Log.Println("Check", id, "deregistered in Consul")
	}
	if !info.Permanent {
		err = c.Client.Agent().ServiceDeregister(string(id))
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
	}
	go cons.checkLoop()
	return cons
}
