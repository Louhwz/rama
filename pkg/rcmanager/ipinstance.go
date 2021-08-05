package rcmanager

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/gogf/gf/container/gset"
	networkingv1 "github.com/oecp/rama/pkg/apis/networking/v1"
	"github.com/oecp/rama/pkg/constants"
	"github.com/oecp/rama/pkg/utils"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog"
)

// reconcile one single node, update/add/remove everything about it's
// corresponding remote vtep
func (m *Manager) reconcileIPInstance(nodeName string) error {
	klog.Infof("Starting reconcile ipinstance from cluster %v, node name=%v", m.ClusterName, nodeName)
	if len(nodeName) == 0 {
		return nil
	}
	vtepName := utils.GenRemoteVtepName(m.ClusterName, nodeName)
	node, err := m.nodeLister.Get(nodeName)
	if err != nil {
		if k8serror.IsNotFound(err) {
			return m.localClusterRamaClient.NetworkingV1().RemoteVteps().Delete(context.TODO(), vtepName, metav1.DeleteOptions{})
		}
		return err
	}
	instances, err := m.IPLister.List(labels.SelectorFromSet(labels.Set{constants.LabelNode: nodeName}))
	if err != nil {
		return err
	}
	// todo maybe need create remote vtep
	remoteVtep, err := m.remoteVtepLister.Get(vtepName)
	if err != nil {
		if k8serror.IsNotFound(err) {
			return nil
		}
		return err
	}
	remoteVtep = remoteVtep.DeepCopy()

	desired := func() []string {
		ipList := make([]string, 0)
		for _, v := range instances {
			if v.Status.Phase == networkingv1.IPPhaseReserved {
				continue
			}
			ip, _, _ := net.ParseCIDR(v.Spec.Address.IP)
			ipList = append(ipList, ip.String())
		}
		return ipList
	}()
	actual := remoteVtep.Status.PodIPList
	desiredSet := gset.NewFrom(desired)
	actualSet := gset.NewFrom(actual)

	remove := actualSet.Diff(desiredSet)
	add := desiredSet.Diff(actualSet)
	actualSet.Remove(remove)
	actualSet.Add(add)

	vtepIP := node.Annotations[constants.AnnotationNodeVtepIP]
	vtepMac := node.Annotations[constants.AnnotationNodeVtepMac]
	vtepChanged := vtepIP != "" && vtepMac != "" && (vtepIP != remoteVtep.Spec.VtepIP || vtepMac != remoteVtep.Spec.VtepMAC)
	if remove.Size() == 0 && add.Size() == 0 && !vtepChanged {
		return nil
	}
	remoteVtep.Spec.VtepIP = vtepIP
	remoteVtep.Spec.VtepMAC = vtepMac
	remoteVtep.Status.LastModifyTime = metav1.NewTime(time.Now())
	remoteVtep.Status.PodIPList = func() []string {
		ans := make([]string, 0, actualSet.Size())
		for _, v := range actualSet.Slice() {
			ans = append(ans, fmt.Sprint(v))
		}
		return ans
	}()

	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		_, err := m.localClusterRamaClient.NetworkingV1().RemoteVteps().Update(context.TODO(), remoteVtep, metav1.UpdateOptions{})
		return err
	})
	return err
}

func (m *Manager) nodeToIPInstance(nodeName string) ([]*networkingv1.IPInstance, error) {
	podIP, err := m.IPIndexer.ByIndex(ByNodeNameIndexer, nodeName)
	if err != nil {
		klog.Warningf("[nodeToPodIP] can't use ipinstance indexer. indexername=%v, nodename=%v, err=%v", ByNodeNameIndexer, nodeName, err)
		return nil, err
	}
	ans := make([]*networkingv1.IPInstance, 0)
	for _, v := range podIP {
		if instance, ok := v.(*networkingv1.IPInstance); ok {
			ans = append(ans, instance)
		}
	}
	return ans, nil
}

func (m *Manager) nodeToIPList(nodeName string) ([]string, error) {
	instances, err := m.nodeToIPInstance(nodeName)
	if err != nil {
		return nil, err
	}
	ipList := make([]string, 0)
	for _, instance := range instances {
		if instance.Status.Phase != networkingv1.IPPhaseUsing {
			continue
		}
		ip, _, _ := net.ParseCIDR(instance.Spec.Address.IP)
		ipList = append(ipList, ip.String())
	}
	return ipList, nil
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
			return fmt.Errorf("[ipinstance] fail to sync '%s' for cluster id=%v: %v, requeuing", key, m.ClusterName, err)
		}
		m.IPQueue.Forget(obj)
		klog.Infof("[ipinstance] succeed to sync '%s', cluster id=%v", key, m.ClusterName)
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

func (m *Manager) addOrDelIPInstance(obj interface{}) {
	ipInstance, _ := obj.(*networkingv1.IPInstance)
	m.enqueueIPInstance(ipInstance.Status.NodeName)
}

func (m *Manager) updateIPInstance(oldObj, newObj interface{}) {
	oldInstance, _ := oldObj.(*networkingv1.IPInstance)
	newInstance, _ := newObj.(*networkingv1.IPInstance)

	if oldInstance.ResourceVersion == newInstance.ResourceVersion {
		return
	}
	if newInstance.Status.Phase == networkingv1.IPPhaseReserved && oldInstance.Status.Phase == networkingv1.IPPhaseReserved {
		return
	}
	if newInstance.Spec.Address.IP != oldInstance.Spec.Address.IP ||
		newInstance.Status.NodeName != oldInstance.Status.NodeName {
		m.enqueueIPInstance(oldInstance.Status.NodeName)
		m.enqueueIPInstance(newInstance.Status.NodeName)
	}
	return
}

func (m *Manager) enqueueIPInstance(instanceName string) {
	m.IPQueue.Add(instanceName)
}
