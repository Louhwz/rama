package rcmanager

import (
	"context"
	"fmt"
	"k8s.io/klog"

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
	//
	//subnets, err := m.subnetLister.List(nil)
	//if err != nil {
	//	return err
	//}
	//rcSubnets, err := m.remoteSubnetLister.List(nil)
	//if err != nil {
	//	return err
	//}
	//
	//
	//
	//update, add, remove := m.diffSubnetAndRCSubnet(subnets, rcSubnets)

	return nil
}

//func (m *Manager) subnetToRemoteSubnet(subnet *networkingv1.Subnet) (*networkingv1.RemoteSubnet, error) {
//	if subnet == nil {
//		return nil, nil
//	}
//	rs := &networkingv1.RemoteSubnet{
//		ObjectMeta: metav1.ObjectMeta{
//			Name: GenRemoteSubnetName(m.clusterID, subnet.Name),
//		},
//		Spec: networkingv1.RemoteSubnetSpec{
//			Version:      subnet.Spec.Range.Version,
//			CIDR:         subnet.Spec.Range.CIDR,
//			Type:         "",
//			ClusterID:    m.clusterID,
//			OverlayNetID: nil,
//		},
//		Status: networkingv1.RemoteSubnetStatus{
//
//		},
//	}
//	network, err :=  m.networkLister.Get(subnet.Spec.Network)
//	if err != nil {
//		return nil, err
//	}
//
//}

func GenRemoteSubnetName(clusterID uint32, subnetName string) string {
	return fmt.Sprintf(rcSubnetNameFormat, clusterID, subnetName)
}

func (m *Manager) removeRCSubnet(subnetName string) error {
	name := GenRemoteSubnetName(m.clusterID, subnetName)

	// todo really work?
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		err := m.inClusterRamaClient.NetworkingV1().RemoteClusters().Delete(context.TODO(), name, metav1.DeleteOptions{})
		return err
	})
}

func (m *Manager) updateRCSubnet(rcSubnet *networkingv1.RemoteSubnet) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		_, err := m.inClusterRamaClient.NetworkingV1().RemoteSubnets().Update(context.TODO(), rcSubnet, metav1.UpdateOptions{})
		return err
	})
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

func (m *Manager) processNextSubnet() bool {
	obj, shutdown := m.subnetQueue.Get()
	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer m.subnetQueue.Done(obj)
		var (
			key string
			ok  bool
		)
		if key, ok = obj.(string); !ok {
			m.subnetQueue.Forget(obj)
			return nil
		}
		if err := m.reconcileSubnet(key); err != nil {
			// TODO: use retry handler to
			// Put the item back on the workqueue to handle any transient errors
			m.subnetQueue.AddRateLimited(key)
			return fmt.Errorf("[subnet] fail to sync '%s' for cluster id=%v: %v, requeuing", key, m.clusterID, err)
		}
		m.subnetQueue.Forget(obj)
		klog.Infof("[subnet] succeed to sync '%s', cluster id=%v", key, m.clusterID)
		return nil
	}(obj)

	if err != nil {
		klog.Error(err)
	}

	return true
}
