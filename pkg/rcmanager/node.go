package rcmanager

import (
	"context"
	"fmt"

	"github.com/oecp/rama/pkg/constants"
	"github.com/oecp/rama/pkg/utils"
	apiv1 "k8s.io/api/core/v1"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
)

func (m *Manager) reconcileNode(key string) error {
	klog.Infof("Starting reconcile node from cluster %v, node name=%v", m.clusterID, key)
	if len(key) == 0 {
		return nil
	}
	_, err := m.nodeLister.Get(key)
	if err != nil {
		if k8serror.IsNotFound(err) {
			name := utils.GenRemoteVtepName(m.clusterID, key)
			err = m.localClusterKubeClient.CoreV1().Nodes().Delete(context.TODO(), name, metav1.DeleteOptions{})
			if k8serror.IsNotFound(err) {
				return nil
			}
			return err
		}
		return err
	}
	return nil
}

func (m *Manager) processNextNode() bool {
	obj, shutdown := m.nodeQueue.Get()
	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer m.nodeQueue.Done(obj)
		var (
			key string
			ok  bool
		)
		if key, ok = obj.(string); !ok {
			m.nodeQueue.Forget(obj)
			return nil
		}
		if err := m.reconcileNode(key); err != nil {
			// TODO: use retry handler to
			// Put the item back on the workqueue to handle any transient errors
			m.nodeQueue.AddRateLimited(key)
			return fmt.Errorf("[node] fail to sync '%s' for cluster id=%v: %v, requeuing", key, m.clusterID, err)
		}
		m.nodeQueue.Forget(obj)
		klog.Infof("[node] succeed to sync '%s', cluster id=%v", key, m.clusterID)
		return nil
	}(obj)

	if err != nil {
		klog.Error(err)
	}

	return true

}

func (m *Manager) RunNodeWorker() {
	for m.processNextNode() {
	}
}

func (m *Manager) filterNode(obj interface{}) bool {
	_, ok := obj.(*apiv1.Node)
	return ok
}

func (m *Manager) addOrDelNode(obj interface{}) {
	node, _ := obj.(*apiv1.Node)
	m.enqueueNode(node.Name)
}

func (m *Manager) updateNode(oldObj, newObj interface{}) {
	oldNode, _ := oldObj.(*apiv1.Node)
	newNode, _ := newObj.(*apiv1.Node)
	newNodeLabels := newNode.Labels
	oldNodeLabels := oldNode.Labels

	if newNodeLabels[constants.AnnotationNodeVtepIP] == "" || newNodeLabels[constants.AnnotationNodeVtepMac] == "" {
		return
	}
	if newNodeLabels[constants.AnnotationNodeVtepIP] == oldNodeLabels[constants.AnnotationIP] &&
		newNodeLabels[constants.AnnotationNodeVtepIP] == oldNodeLabels[constants.AnnotationNodeVtepMac] {
		return
	}
	m.enqueueNode(newNode.Name)
}

func (m *Manager) enqueueNode(nodeName string) {
	m.nodeQueue.Add(nodeName)
}
