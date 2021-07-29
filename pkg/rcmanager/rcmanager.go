package rcmanager

import (
	"context"
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
	clusterStatus       *apiv1.RemoteClusterStatus
	inClusterKubeClient kubeclientset.Interface
	inClusterRamaClient versioned.Interface
	remoteSubnetLister  listers.RemoteSubnetLister
	remoteSubnetSynced  cache.InformerSynced

	KubeClient          *kubeclientset.Clientset
	RamaClient          *versioned.Clientset
	KubeInformerFactory informers.SharedInformerFactory
	RamaInformerFactory externalversions.SharedInformerFactory
	nodeLister          corev1.NodeLister
	NodeSynced          cache.InformerSynced
	nodeQueue           workqueue.RateLimitingInterface
	networkLister       listers.NetworkLister
	networkSynced       cache.InformerSynced
	subnetLister        listers.SubnetLister
	SubnetSynced        cache.InformerSynced
	subnetQueue         workqueue.RateLimitingInterface
	ipLister            listers.IPInstanceLister
	IpSynced            cache.InformerSynced
	ipQueue             workqueue.RateLimitingInterface
}

func NewRemoteClusterManager(
	localClusterKubeClient kubeclientset.Interface,
	rc *apiv1.RemoteCluster,
	remoteSubnetLister listers.RemoteSubnetLister,
	remoteSubnetHasSynced cache.InformerSynced) (*Manager, error) {
	defer func() {
		if err := recover(); err != nil {
			klog.Warningf("Panic hanppened. Maybe wrong kube config. err=%v", err)
		}
	}()

	config, err := utils.BuildClusterConfig(localClusterKubeClient, rc)
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
	rcManager.remoteSubnetLister = remoteSubnetLister
	rcManager.remoteSubnetSynced = remoteSubnetHasSynced

	rcManager.KubeInformerFactory = informers.NewSharedInformerFactory(rcManager.KubeClient, 0)
	rcManager.RamaInformerFactory = externalversions.NewSharedInformerFactory(rcManager.RamaClient, 0)
	rcManager.nodeLister = rcManager.KubeInformerFactory.Core().V1().Nodes().Lister()
	rcManager.NodeSynced = rcManager.KubeInformerFactory.Core().V1().Nodes().Informer().HasSynced
	rcManager.subnetLister = rcManager.RamaInformerFactory.Networking().V1().Subnets().Lister()
	rcManager.SubnetSynced = rcManager.RamaInformerFactory.Networking().V1().Subnets().Informer().HasSynced
	rcManager.ipLister = rcManager.RamaInformerFactory.Networking().V1().IPInstances().Lister()
	rcManager.IpSynced = rcManager.RamaInformerFactory.Networking().V1().IPInstances().Informer().HasSynced
	rcManager.networkLister = rcManager.RamaInformerFactory.Networking().V1().Networks().Lister()
	rcManager.networkSynced = rcManager.RamaInformerFactory.Networking().V1().Networks().Informer().HasSynced

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
	rcManager.inClusterKubeClient = localClusterKubeClient
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

func (m *Manager) RunNodeWorker() {

}

func (m *Manager) RunIPInstanceWorker() {

}

func (m *Manager) RunSubnetWorker() {
	for m.processNextSubnet() {
	}
}

func (m *Manager) filterIpInstance(obj interface{}) bool {
	_, ok := obj.(*apiv1.IPInstance)
	return ok
}

func (m *Manager) diffSubnetAndRCSubnet(subnets []*apiv1.Subnet, subnets2 []*apiv1.RemoteSubnet) (interface{}, interface{}, interface{}) {
	return nil, nil, nil
}
