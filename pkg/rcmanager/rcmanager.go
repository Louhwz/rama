package rcmanager

import (
	"fmt"
	"runtime/debug"
	"sync"

	jsoniter "github.com/json-iterator/go"
	networkingv1 "github.com/oecp/rama/pkg/apis/networking/v1"
	"github.com/oecp/rama/pkg/client/clientset/versioned"
	"github.com/oecp/rama/pkg/client/informers/externalversions"
	listers "github.com/oecp/rama/pkg/client/listers/networking/v1"
	"github.com/oecp/rama/pkg/utils"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/informers"
	kubeclientset "k8s.io/client-go/kubernetes"
	corev1 "k8s.io/client-go/listers/core/v1"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"
)

const (
	UserAgentName = "RemoteClusterManager"

	// ByNodeNameIndexer nodeName to ipinstance indexer
	ByNodeNameIndexer = "nodename"
)

// Manager Those without the localCluster prefix are the resources of the remote cluster
type Manager struct {
	Meta
	localClusterKubeClient   kubeclientset.Interface
	localClusterRamaClient   versioned.Interface
	remoteSubnetLister       listers.RemoteSubnetLister
	remoteVtepLister         listers.RemoteVtepLister
	localClusterSubnetLister listers.SubnetLister

	KubeClient              *kubeclientset.Clientset
	RamaClient              *versioned.Clientset
	KubeInformerFactory     informers.SharedInformerFactory
	RamaInformerFactory     externalversions.SharedInformerFactory
	nodeLister              corev1.NodeLister
	NodeSynced              cache.InformerSynced
	nodeQueue               workqueue.RateLimitingInterface
	networkLister           listers.NetworkLister
	networkSynced           cache.InformerSynced
	subnetLister            listers.SubnetLister
	SubnetSynced            cache.InformerSynced
	subnetQueue             workqueue.RateLimitingInterface
	IPLister                listers.IPInstanceLister
	IPSynced                cache.InformerSynced
	IPQueue                 workqueue.RateLimitingInterface
	IPIndexer               cache.Indexer
	remoteClusterNodeLister corev1.NodeLister
	remoteClusterNodeSynced cache.InformerSynced
}

type Meta struct {
	ClusterName string
	UID         types.UID
	StopCh      chan struct{}
	// Only if meet the condition, can create remote cluster's cr
	// Conditions are:
	// 1. The remote cluster created the remote-cluster-cr of this cluster
	// 2. The remote cluster and local cluster both have overlay network
	// 3. The overlay network id is same with local cluster
	MeetCondition     bool
	MeetConditionLock sync.RWMutex
}

func (m *Manager) GetMeetCondition() bool {
	m.MeetConditionLock.RLock()
	defer m.MeetConditionLock.RUnlock()

	return m.MeetCondition
}

func NewRemoteClusterManager(rc *networkingv1.RemoteCluster,
	localClusterKubeClient kubeclientset.Interface,
	localClusterRamaClient versioned.Interface,
	remoteSubnetLister listers.RemoteSubnetLister,
	localClusterSubnetLister listers.SubnetLister,
	remoteVtepLister listers.RemoteVtepLister) (*Manager, error) {
	defer func() {
		if err := recover(); err != nil {
			s, _ := jsoniter.MarshalToString(rc)
			klog.Errorf("Panic hanppened. Can't new remote cluster manager. Maybe wrong kube config. "+
				"err=%v. remote cluster=%v\n%v", err, s, debug.Stack())
		}
	}()
	klog.Infof("NewRemoteClusterManager %v", rc.Name)

	config, err := utils.BuildClusterConfig(rc)
	if err != nil {
		return nil, err
	}

	kubeClient := kubeclientset.NewForConfigOrDie(config)
	ramaClient := versioned.NewForConfigOrDie(restclient.AddUserAgent(config, UserAgentName))
	kubeInformerFactory := informers.NewSharedInformerFactory(kubeClient, 0)
	ramaInformerFactory := externalversions.NewSharedInformerFactory(ramaClient, 0)

	nodeInformer := kubeInformerFactory.Core().V1().Nodes()
	networkInformer := ramaInformerFactory.Networking().V1().Networks()
	subnetInformer := ramaInformerFactory.Networking().V1().Subnets()
	ipInformer := ramaInformerFactory.Networking().V1().IPInstances()

	if err := ipInformer.Informer().GetIndexer().AddIndexers(cache.Indexers{
		ByNodeNameIndexer: indexByNodeName,
	}); err != nil {
		klog.Errorf("index by node name failed. err=%v. ", err)
		return nil, errors.New("Can't add indexer")
	}
	stopCh := make(chan struct{})

	rcMgr := &Manager{
		Meta: Meta{
			ClusterName:   rc.Name,
			UID:           rc.UID,
			StopCh:        stopCh,
			MeetCondition: false,
		},
		localClusterKubeClient:   localClusterKubeClient,
		localClusterRamaClient:   localClusterRamaClient,
		remoteSubnetLister:       remoteSubnetLister,
		localClusterSubnetLister: localClusterSubnetLister,
		remoteVtepLister:         remoteVtepLister,
		KubeClient:               kubeClient,
		RamaClient:               ramaClient,
		KubeInformerFactory:      kubeInformerFactory,
		RamaInformerFactory:      ramaInformerFactory,
		nodeLister:               kubeInformerFactory.Core().V1().Nodes().Lister(),
		NodeSynced:               kubeInformerFactory.Core().V1().Nodes().Informer().HasSynced,
		nodeQueue:                workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), fmt.Sprintf("%v-node", rc.ClusterName)),
		networkLister:            ramaInformerFactory.Networking().V1().Networks().Lister(),
		networkSynced:            networkInformer.Informer().HasSynced,
		subnetLister:             ramaInformerFactory.Networking().V1().Subnets().Lister(),
		SubnetSynced:             subnetInformer.Informer().HasSynced,
		subnetQueue:              workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), fmt.Sprintf("%v-subnet", rc.ClusterName)),
		IPLister:                 ramaInformerFactory.Networking().V1().IPInstances().Lister(),
		IPSynced:                 ipInformer.Informer().HasSynced,
		IPQueue:                  workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), fmt.Sprintf("%v-ipinstance", rc.ClusterName)),
		IPIndexer:                ipInformer.Informer().GetIndexer(),
		remoteClusterNodeLister:  kubeInformerFactory.Core().V1().Nodes().Lister(),
		remoteClusterNodeSynced:  nodeInformer.Informer().HasSynced,
	}

	nodeInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: rcMgr.filterNode,
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    rcMgr.addOrDelNode,
			UpdateFunc: rcMgr.updateNode,
			DeleteFunc: rcMgr.addOrDelNode,
		},
	})

	subnetInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: rcMgr.filterSubnet,
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    rcMgr.addOrDelSubnet,
			UpdateFunc: rcMgr.updateSubnet,
			DeleteFunc: rcMgr.addOrDelSubnet,
		},
	})

	ipInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: rcMgr.filterIPInstance,
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    rcMgr.addOrDelIPInstance,
			UpdateFunc: rcMgr.updateIPInstance,
			DeleteFunc: rcMgr.addOrDelIPInstance,
		},
	})
	klog.Infof("Successfully New Remote Cluster Manager. Cluster=%v", rc.Name)
	return rcMgr, nil
}

func indexByNodeName(obj interface{}) ([]string, error) {
	instance, ok := obj.(*networkingv1.IPInstance)
	if ok {
		return []string{instance.Status.NodeName}, nil
	}
	return []string{}, nil
}
