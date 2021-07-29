package rcmanager

import (
	"context"
	"fmt"
	"strings"
	"time"

	apiv1 "github.com/oecp/rama/pkg/apis/networking/v1"
	"github.com/oecp/rama/pkg/client/clientset/versioned"
	"github.com/oecp/rama/pkg/client/informers/externalversions"
	listers "github.com/oecp/rama/pkg/client/listers/networking/v1"
	"github.com/oecp/rama/pkg/utils"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/informers"
	kubeclientset "k8s.io/client-go/kubernetes"
	corev1 "k8s.io/client-go/listers/core/v1"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"
)

const UserAgentName = "Cluster-Controller"

type Manager struct {
	clusterID           uint32
	clusterName         string
	inClusterKubeClient kubeclientset.Interface
	inClusterRamaClient versioned.Interface
	KubeClient          *kubeclientset.Clientset
	RamaClient          *versioned.Clientset
	KubeInformerFactory informers.SharedInformerFactory
	RamaInformerFactory externalversions.SharedInformerFactory
	nodeLister          corev1.NodeLister
	NodeSynced          cache.InformerSynced
	nodeQueue           workqueue.RateLimitingInterface
	subnetLister        listers.SubnetLister
	SubnetSynced        cache.InformerSynced
	subnetQueue         workqueue.RateLimitingInterface
	ipLister            listers.IPInstanceLister
	IpSynced            cache.InformerSynced
	ipQueue             workqueue.RateLimitingInterface
	clusterStatus       *apiv1.RemoteClusterStatus
}

func NewRemoteClusterManager(client kubeclientset.Interface, rc *apiv1.RemoteCluster) (*Manager, error) {
	defer func() {
		if err := recover(); err != nil {
			klog.Warningf("Panic hanppened. Maybe wrong kube config. err=%v", err)
		}
	}()

	config, err := utils.BuildClusterConfig(client, rc)
	if err != nil {
		return nil, err
	}
	rcManager := &Manager{
		clusterID:   rc.Spec.ClusterID,
		clusterName: rc.Spec.ClusterName,
	}
	config.Timeout = time.Duration(rc.Spec.ConnConfig.Timeout) * time.Second

	rcManager.KubeClient = kubeclientset.NewForConfigOrDie(config)
	rcManager.RamaClient = versioned.NewForConfigOrDie(restclient.AddUserAgent(config, UserAgentName))

	rcManager.KubeInformerFactory = informers.NewSharedInformerFactory(rcManager.KubeClient, 0)
	rcManager.RamaInformerFactory = externalversions.NewSharedInformerFactory(rcManager.RamaClient, 0)
	rcManager.nodeLister = rcManager.KubeInformerFactory.Core().V1().Nodes().Lister()
	rcManager.NodeSynced = rcManager.KubeInformerFactory.Core().V1().Nodes().Informer().HasSynced
	rcManager.subnetLister = rcManager.RamaInformerFactory.Networking().V1().Subnets().Lister()
	rcManager.SubnetSynced = rcManager.RamaInformerFactory.Networking().V1().Subnets().Informer().HasSynced
	rcManager.ipLister = rcManager.RamaInformerFactory.Networking().V1().IPInstances().Lister()
	rcManager.IpSynced = rcManager.RamaInformerFactory.Networking().V1().IPInstances().Informer().HasSynced

	nodeInformer := rcManager.KubeInformerFactory.Core().V1().Nodes().Informer()
	nodeInformer.AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: nil,
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    nil,
			UpdateFunc: nil,
			DeleteFunc: nil,
		},
	})
	subnetInformer := rcManager.RamaInformerFactory.Networking().V1().RemoteSubnets().Informer()
	subnetInformer.AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: rcManager.filterSubnet,
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    rcManager.addOrDelSubnet,
			UpdateFunc: rcManager.updateSubnet,
			DeleteFunc: rcManager.addOrDelSubnet,
		},
	})
	ipInformer := rcManager.RamaInformerFactory.Networking().V1().IPInstances().Informer()
	ipInformer.AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: nil,
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    nil,
			UpdateFunc: nil,
			DeleteFunc: nil,
		},
	})
	rcManager.inClusterKubeClient = client
	return rcManager, nil
}

func (m *Manager) getClusterHealthStatus() (*apiv1.RemoteClusterStatus, error) {
	clusterStatus := &apiv1.RemoteClusterStatus{}

	body, err := m.KubeClient.DiscoveryClient.RESTClient().Get().AbsPath("/healthz").Do(context.TODO()).Raw()
	if err != nil {
		runtime.HandleError(errors.Wrapf(err, "Cluster Health Check failed for cluster %v", m.clusterID))
		m.clusterStatus.Conditions = append(m.clusterStatus.Conditions)
	} else {
		if !strings.EqualFold(string(body), "ok") {

		} else {

		}
	}
	return clusterStatus, nil
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

func (m *Manager) RunNodeWorker() {

}

func (m *Manager) RunIPInstanceWorker() {

}

func (m *Manager) filterIpInstance(obj interface{}) bool {
	_, ok := obj.(*apiv1.IPInstance)
	return ok
}
