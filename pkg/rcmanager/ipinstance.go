package rcmanager

import (
	"fmt"

	networkingv1 "github.com/oecp/rama/pkg/apis/networking/v1"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
)

func (m *Manager) reconcileIPInstance(key string) error {
	klog.Infof("Starting reconcile ipinstance from cluster %v, ipinstance name=%v", m.clusterID, key)
	if len(key) == 0 {
		return nil
	}
	_, err := m.IPLister.IPInstances(metav1.NamespaceAll).Get(key)
	if err != nil {
		if k8serror.IsNotFound(err) {
			//nodeName := instance.Status.NodeName
			//nodeName 2 remote vtep
			//remote vtep ip list delete ipinstance ip
			//save remote vtep
			//
			//
			//err = m.localClusterRamaClient.NetworkingV1().RemoteSubnets().Delete(context.TODO(), name, metav1.DeleteOptions{})
			//if k8serror.IsNotFound(err) {
			//	return nil
			//}
			return err
		}
		return err
	}

	return nil
}

func (m *Manager) RunIPInstanceWorker() {
	for m.processNextIPInstance() {
	}
}

func (m *Manager) processNextIPInstance() bool {
	obj, shutdown := m.IPQueue.Get()
	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer m.IPQueue.Done(obj)
		var (
			key string
			ok  bool
		)
		if key, ok = obj.(string); !ok {
			m.IPQueue.Forget(obj)
			return nil
		}
		if err := m.reconcileIPInstance(key); err != nil {
			// TODO: use retry handler to
			// Put the item back on the workqueue to handle any transient errors
			m.IPQueue.AddRateLimited(key)
			return fmt.Errorf("[ipinstance] fail to sync '%s' for cluster id=%v: %v, requeuing", key, m.clusterID, err)
		}
		m.IPQueue.Forget(obj)
		klog.Infof("[ipinstance] succeed to sync '%s', cluster id=%v", key, m.clusterID)
		return nil
	}(obj)

	if err != nil {
		klog.Error(err)
	}
	return true
}

func (m *Manager) filterIPInstance(obj interface{}) bool {
	_, ok := obj.(*networkingv1.IPInstance)
	return ok
}

func (m *Manager) addIPInstance(obj interface{}) {
	ipInstance, _ := obj.(*networkingv1.IPInstance)
	m.enqueueIPInstance(ipInstance.Name)
}

func (m *Manager) updateIPInstance(oldObj, newObj interface{}) {
	oldRC, _ := oldObj.(*networkingv1.Subnet)
	newRC, _ := newObj.(*networkingv1.Subnet)

	if oldRC.ResourceVersion == newRC.ResourceVersion {
		return
	}
	m.enqueueIPInstance(newRC.ObjectMeta.Name)
}

func (m *Manager) enqueueIPInstance(instanceName string) {
	m.IPQueue.Add(instanceName)
}
