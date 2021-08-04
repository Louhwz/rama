package remotecluster

import (
	"fmt"
	"reflect"

	v1 "github.com/oecp/rama/pkg/apis/networking/v1"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog"
)

func (c *Controller) filterRemoteCluster(obj interface{}) bool {
	_, ok := obj.(*v1.RemoteCluster)
	return ok
}

func (c *Controller) addOrDelRemoteCluster(obj interface{}) {
	rc, _ := obj.(*v1.RemoteCluster)
	c.enqueueRemoteCluster(rc.Name)
}

func (c *Controller) updateRemoteCluster(oldObj, newObj interface{}) {
	oldRC, _ := oldObj.(*v1.RemoteCluster)
	newRC, _ := newObj.(*v1.RemoteCluster)

	if oldRC.ResourceVersion == newRC.ResourceVersion ||
		oldRC.Generation == newRC.Generation {
		return
	}
	if !remoteClusterSpecChanged(&oldRC.Spec, &newRC.Spec) {
		return
	}
	c.enqueueRemoteCluster(newRC.ClusterName)
}

func (c *Controller) enqueueRemoteCluster(clusterName string) {
	c.remoteClusterQueue.Add(clusterName)
}

// remote cluster is managed by admin, no need to full synchronize
func (c *Controller) reconcileRemoteCluster(clusterName string) error {
	remoteCluster, err := c.remoteClusterLister.Get(clusterName)
	if err != nil {
		if k8serror.IsNotFound(err) {
			return nil
		}
		return err
	}
	return c.addOrUpdateRemoteClusterManager(remoteCluster)
}

func (c *Controller) delRemoteCluster(clusterName string) {
	klog.Infof("deleting clusterID=%v.", clusterName)
	c.delRemoteClusterManager(clusterName)
}

func (c *Controller) runRemoteClusterWorker() {
	for c.processNextRemoteCluster() {
	}
}

func (c *Controller) processNextRemoteCluster() bool {
	obj, shutdown := c.remoteClusterQueue.Get()
	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer c.remoteClusterQueue.Done(obj)
		var (
			key string
			ok  bool
		)
		if key, ok = obj.(string); !ok {
			c.remoteClusterQueue.Forget(obj)
			return nil
		}
		if err := c.reconcileRemoteCluster(key); err != nil {
			// TODO: use retry handler to
			// Put the item back on the workqueue to handle any transient errors
			c.remoteClusterQueue.AddRateLimited(key)
			return fmt.Errorf("[remote cluster] fail to sync '%v': %v, requeuing", key, err)
		}
		c.remoteClusterQueue.Forget(obj)
		klog.Infof("[remote cluster] succeed to sync '%v'", key)
		return nil
	}(obj)

	if err != nil {
		klog.Error(err)
	}

	return true
}

func remoteClusterSpecChanged(old, new *v1.RemoteClusterSpec) bool {
	return !reflect.DeepEqual(old.ConnConfig, new.ConnConfig)
}
