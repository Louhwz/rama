package rcmanager

import (
	"context"
	"fmt"

	networkingv1 "github.com/oecp/rama/pkg/apis/networking/v1"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

const rcSubnetNameFormat = "cluster%v.%v"

func (m *Manager) reconcileSubnet(key string) error {
	if len(key) == 0 {
		return nil
	}
	_, err := m.subnetLister.Get(key)
	if err != nil {
		if k8serror.IsNotFound(err) {
			return m.removeRCSubnet(key)
		}
		return err
	}
	return nil
}

func GenRemoteSubnetName(clusterID uint32, subnetName string) string {
	return fmt.Sprintf(rcSubnetNameFormat, clusterID, subnetName)
}

func (m *Manager) removeRCSubnet(subnetName string) error {
	name := GenRemoteSubnetName(m.clusterID, subnetName)

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		err := m.inClusterRamaClient.NetworkingV1().RemoteClusters().Delete(context.TODO(), name, metav1.DeleteOptions{})
		return err
	})
}

func (m *Manager) runSubnetWorker() {
	for m.processNextSubnet() {
	}
}

func (m *Manager) filterSubnet(obj interface{}) bool {
	_, ok := obj.(*networkingv1.RemoteSubnet)
	return ok
}

func (m *Manager) addOrDelSubnet(obj interface{}) {
	subnet, _ := obj.(*networkingv1.Subnet)
	m.enqueueSubnet(subnet.ObjectMeta.Name)
}

func (m *Manager) updateSubnet(oldObj, newObj interface{}) {
	oldRC, _ := oldObj.(*networkingv1.Subnet)
	newRC, _ := newObj.(*networkingv1.Subnet)

	if oldRC.ResourceVersion == newRC.ResourceVersion {
		return
	}
	if oldRC.Generation == newRC.Generation {
		return
	}
	m.enqueueSubnet(newRC.ObjectMeta.Name)
}

func (m *Manager) enqueueSubnet(subnetName string) {
	m.subnetQueue.Add(subnetName)
}
