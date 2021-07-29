package remotecluster

import (
	"sync"
)

type Cache struct {
	mu               sync.RWMutex
	remoteClusterMap map[uint32]*Manager
}

func (c *Cache) Get(clusterID uint32) (manager *Manager, exists bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	manager, exists = c.remoteClusterMap[clusterID]
	return
}

func (c *Cache) Set(key uint32, manager *Manager) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.remoteClusterMap[key] = manager
}
