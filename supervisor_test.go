package container

import (
	"testing"
	"context"
	"github.com/stretchr/testify/assert"
	"log"
	"os"
)

func TestNewSupervisor(t *testing.T) {
	su := NewSupervisor(log.New(os.Stderr, "[supervisor] ", log.LstdFlags))
	spawned := false
	id, done, _ := su.SpawnFunc("", func(ctx context.Context) error {
		spawned = true
		return nil
	})
	t.Log("ID", id)
	err := <-done
	assert.True(t, spawned, "Spawned")
	assert.NoError(t, err, "Return error")
}
