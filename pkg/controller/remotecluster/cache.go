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

	//def QuickSort(nums):
	//quick(nums, 0, len(nums) - 1)
	//
	//
	//def quick(nums, start, end):
	//if start >= end:
	//return
	//pivot = nums[start]
	//i, j = start, end
	//while i < j:
	//while i < j and nums[i] <= pivot:
	//i = i + 1
	//nums[i], nums[j] = nums[j], nums[i]
	//while i < j and nums[j] >= pivot:
	//j = j - 1
	//nums[i], nums[j] = nums[j], nums[i]
	//nums[i] = pivot
	//quick(nums, start, i - 1)
	//quick(nums, i + 1, end)
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
