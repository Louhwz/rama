package remotecluster

import (
	"fmt"
	"reflect"
	"strconv"

	jsoniter "github.com/json-iterator/go"
	v1 "github.com/oecp/rama/pkg/apis/networking/v1"
	"gopkg.in/errgo.v2/fmt/errors"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog"
)

func (c *Controller) filterRemoteCluster(obj interface{}) bool {
	_, ok := obj.(*v1.RemoteCluster)
	return ok
}

func (c *Controller) addRemoteCluster(obj interface{}) {
	rc, _ := obj.(*v1.RemoteCluster)
	c.enqueueRemoteCluster(rc.Spec.ClusterID)
}

func (c *Controller) deleteRemoteCluster(obj interface{}) {
	rc, _ := obj.(*v1.RemoteCluster)
	c.enqueueRemoteCluster(rc.Spec.ClusterID)
}

func (c *Controller) updateRemoteCluster(oldObj, newObj interface{}) {
	oldRC, _ := oldObj.(*v1.RemoteCluster)
	newRC, _ := newObj.(v1.RemoteCluster)

	if oldRC.ResourceVersion == newRC.ResourceVersion {
		return
	}
	if oldRC.Generation == newRC.Generation {
		return
	}
	if !remoteClusterSpecChanged(&oldRC.Spec, &newRC.Spec) {
		return
	}
	c.enqueueRemoteCluster(newRC.Spec.ClusterID)
}

func (c *Controller) enqueueRemoteCluster(clusterID uint32) {
	c.remoteClusterQueue.Add(clusterID)
}

func (c *Controller) reconcileRemoteCluster(key uint32) error {
	clusterIDStr := strconv.FormatInt(int64(key), 10)
	remoteCluster, err := c.remoteClusterIndexer.ByIndex(ByRemoteClusterIDIndexer, clusterIDStr)
	switch {
	case err != nil && k8serror.IsNotFound(err):
		return c.delRemoteClusterClient(key)
	case len(remoteCluster) != 1:
		return errors.Newf("get more than one cluster for one cluster id. key=%v", key)
	case len(remoteCluster) == 0:
		return nil
	}
	rc, ok := remoteCluster[0].(*v1.RemoteCluster)
	if !ok {
		s, _ := jsoniter.MarshalToString(remoteCluster)
		klog.Errorf("Can't assertion to remote cluster. value=%v", s)
		return errors.New("Can't assertion")
	}

	return c.addOrUpdateRemoteClusterManager(rc)

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
			key uint32
			ok  bool
		)
		if key, ok = obj.(uint32); !ok {
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

func indexByRemoteClusterID(obj interface{}) ([]string, error) {
	instance, ok := obj.(*v1.RemoteCluster)
	if ok {
		clusterID := instance.Spec.ClusterID
		return []string{strconv.FormatInt(int64(clusterID), 10)}, nil
	}
	return []string{}, nil
}
