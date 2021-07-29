package remotecluster

import (
	"sync"

	"github.com/oecp/rama/pkg/rcmanager"
)

type Cache struct {
	mu               sync.RWMutex
	remoteClusterMap map[uint32]*rcmanager.Manager
}

func (c *Cache) Get(clusterID uint32) (manager *rcmanager.Manager, exists bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	manager, exists = c.remoteClusterMap[clusterID]
	return
}

func (c *Cache) Set(key uint32, manager *rcmanager.Manager) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.remoteClusterMap[key] = manager
}

func (c *Cache) Del(key uint32) {
	c.mu.Lock()
	defer c.mu.Unlock()
	// todo any way to release client?
	delete(c.remoteClusterMap, key)
}
