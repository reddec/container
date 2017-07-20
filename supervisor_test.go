package container

import (
	"testing"
	"context"
	"github.com/stretchr/testify/assert"
)

func TestNewSupervisor(t *testing.T) {
	su := NewSupervisor()
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
