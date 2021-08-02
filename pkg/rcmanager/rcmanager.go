package rcmanager

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	networkingv1 "github.com/oecp/rama/pkg/apis/networking/v1"
	"github.com/oecp/rama/pkg/client/clientset/versioned"
	"github.com/oecp/rama/pkg/client/informers/externalversions"
	listers "github.com/oecp/rama/pkg/client/listers/networking/v1"
	"github.com/oecp/rama/pkg/constants"
	"github.com/oecp/rama/pkg/utils"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/informers"
	kubeclientset "k8s.io/client-go/kubernetes"
	corev1 "k8s.io/client-go/listers/core/v1"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"
)

const (
	UserAgentName = "Cluster-Controller"

	ByNodeNameIndexer = "nodename"
)

type Manager struct {
	clusterID                uint32
	clusterName              string
	clusterStatus            *networkingv1.RemoteClusterStatus
	localClusterKubeClient   kubeclientset.Interface
	localClusterRamaClient   versioned.Interface
	remoteSubnetLister       listers.RemoteSubnetLister
	remoteSubnetSynced       cache.InformerSynced
	remoteVtepLister         listers.RemoteVtepLister
	remoteVtepSynced         cache.InformerSynced
	localClusterSubnetLister listers.SubnetLister
	localClusterSubnetSynced cache.InformerSynced

	KubeClient               *kubeclientset.Clientset
	RamaClient               *versioned.Clientset
	KubeInformerFactory      informers.SharedInformerFactory
	RamaInformerFactory      externalversions.SharedInformerFactory
	nodeLister               corev1.NodeLister
	NodeSynced               cache.InformerSynced
	nodeQueue                workqueue.RateLimitingInterface
	networkLister            listers.NetworkLister
	networkSynced            cache.InformerSynced
	subnetLister             listers.SubnetLister
	SubnetSynced             cache.InformerSynced
	subnetQueue              workqueue.RateLimitingInterface
	IPLister                 listers.IPInstanceLister
	IPSynced                 cache.InformerSynced
	IPQueue                  workqueue.RateLimitingInterface
	IPIndexer                cache.Indexer
	remoteClusterNodeLister  corev1.NodeLister
	remoteClusterNodeSynced  cache.InformerSynced
	remoteClusterNodeIndexer cache.Indexer
}

func NewRemoteClusterManager(
	localClusterKubeClient kubeclientset.Interface,
	rc *networkingv1.RemoteCluster,
	remoteSubnetLister listers.RemoteSubnetLister,
	remoteSubnetHasSynced cache.InformerSynced,
	localClusterSubnetLister listers.SubnetLister,
	localClusterSubnetSynced cache.InformerSynced) (*Manager, error) {
	defer func() {
		if err := recover(); err != nil {
			klog.Warningf("Panic hanppened. Maybe wrong kube config. err=%v", err)
		}
	}()

	config, err := utils.BuildClusterConfig(localClusterKubeClient, rc)
	if err != nil {
		return nil, err
	}
	config.Timeout = time.Duration(rc.Spec.ConnConfig.Timeout) * time.Second

	kubeClient := kubeclientset.NewForConfigOrDie(config)
	ramaClient := versioned.NewForConfigOrDie(restclient.AddUserAgent(config, UserAgentName))
	kubeInformerFactory := informers.NewSharedInformerFactory(kubeClient, 0)
	ramaInformerFactory := externalversions.NewSharedInformerFactory(ramaClient, 0)

	if err := ramaInformerFactory.Networking().V1().IPInstances().Informer().GetIndexer().AddIndexers(cache.Indexers{
		ByNodeNameIndexer: indexByNodeName,
	}); err != nil {
		klog.Error("index by node name failed. err=%v. ", err)
		return nil, errors.New("Can't add indexer")
	}

	rcManager := &Manager{
		clusterID:                rc.Spec.ClusterID,
		clusterName:              rc.Spec.ClusterName,
		clusterStatus:            nil,
		localClusterKubeClient:   localClusterKubeClient,
		localClusterRamaClient:   nil,
		remoteSubnetLister:       remoteSubnetLister,
		remoteSubnetSynced:       remoteSubnetHasSynced,
		remoteVtepLister:         nil,
		remoteVtepSynced:         nil,
		localClusterSubnetLister: localClusterSubnetLister,
		localClusterSubnetSynced: localClusterSubnetSynced,
		KubeClient:               kubeClient,
		RamaClient:               ramaClient,
		KubeInformerFactory:      kubeInformerFactory,
		RamaInformerFactory:      ramaInformerFactory,
		nodeLister:               kubeInformerFactory.Core().V1().Nodes().Lister(),
		NodeSynced:               kubeInformerFactory.Core().V1().Nodes().Informer().HasSynced,
		nodeQueue:                workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), fmt.Sprintf("node-%v", rc.Spec.ClusterID)),
		networkLister:            nil,
		networkSynced:            nil,
		subnetLister:             nil,
		SubnetSynced:             nil,
		subnetQueue:              nil,
		IPLister:                 nil,
		IPSynced:                 nil,
		IPQueue:                  nil,
		IPIndexer:                nil,
		remoteClusterNodeLister:  nil,
		remoteClusterNodeSynced:  nil,
		remoteClusterNodeIndexer: nil,
	}

	rcManager.subnetLister = rcManager.RamaInformerFactory.Networking().V1().Subnets().Lister()
	rcManager.SubnetSynced = rcManager.RamaInformerFactory.Networking().V1().Subnets().Informer().HasSynced
	rcManager.subnetQueue = workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), fmt.Sprintf("subnet-%v", rcManager.clusterID))
	rcManager.IPLister = rcManager.RamaInformerFactory.Networking().V1().IPInstances().Lister()
	rcManager.IPSynced = rcManager.RamaInformerFactory.Networking().V1().IPInstances().Informer().HasSynced
	rcManager.IPIndexer = rcManager.RamaInformerFactory.Networking().V1().IPInstances().Informer().GetIndexer()
	rcManager.IPQueue = workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), fmt.Sprintf("ipinstance-%v", rcManager.clusterID))
	rcManager.networkLister = rcManager.RamaInformerFactory.Networking().V1().Networks().Lister()
	rcManager.networkSynced = rcManager.RamaInformerFactory.Networking().V1().Networks().Informer().HasSynced
	rcManager.remoteClusterNodeLister = rcManager.KubeInformerFactory.Core().V1().Nodes().Lister()
	rcManager.remoteClusterNodeSynced = rcManager.KubeInformerFactory.Core().V1().Nodes().Informer().HasSynced

	nodeInformer := rcManager.KubeInformerFactory.Core().V1().Nodes().Informer()
	nodeInformer.AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: rcManager.filterNode,
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    rcManager.addOrDelNode,
			UpdateFunc: rcManager.updateNode,
			DeleteFunc: rcManager.addOrDelNode,
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
		FilterFunc: rcManager.filterIPInstance,
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    rcManager.addIPInstance,
			UpdateFunc: nil,
			DeleteFunc: nil,
		},
	})
	rcManager.localClusterKubeClient = localClusterKubeClient
	return rcManager, nil
}

func (m *Manager) getClusterHealthStatus() (*networkingv1.RemoteClusterStatus, error) {
	clusterStatus := &networkingv1.RemoteClusterStatus{}

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

func (m *Manager) clusterIDSelector() labels.Selector {
	s := labels.Set{
		constants.LabelCluster: strconv.FormatInt(int64(m.clusterID), 10),
	}
	return labels.SelectorFromSet(s)
}

func indexByNodeName(obj interface{}) ([]string, error) {
	instance, ok := obj.(*networkingv1.IPInstance)
	if ok {
		return []string{instance.Status.NodeName}, nil
	}
	return []string{}, nil
}
