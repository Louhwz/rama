package remotecluster

import (
	"sync"

	"github.com/oecp/rama/pkg/rcmanager"
)

type Cache struct {
	mu               sync.RWMutex
	remoteClusterMap map[string]*rcmanager.Manager
}

type ManagerWrapper struct {
	stopCh chan struct{}

	rcmanager.Manager
}

func (c *Cache) Get(clusterName string) (manager *rcmanager.Manager, exists bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	manager, exists = c.remoteClusterMap[clusterName]
	return
}

func (c *Cache) Set(clusterName string, manager *rcmanager.Manager) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.remoteClusterMap[clusterName] = manager
}

func (c *Cache) Del(clusterName string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	// todo any way to release client?
	delete(c.remoteClusterMap, clusterName)
}

func (c *Cache) Clear() {
	//for _,v := range
}
