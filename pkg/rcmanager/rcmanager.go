package rcmanager

import (
	"fmt"
	"time"

	jsoniter "github.com/json-iterator/go"

	networkingv1 "github.com/oecp/rama/pkg/apis/networking/v1"
	"github.com/oecp/rama/pkg/client/clientset/versioned"
	"github.com/oecp/rama/pkg/client/informers/externalversions"
	v1 "github.com/oecp/rama/pkg/client/informers/externalversions/networking/v1"
	listers "github.com/oecp/rama/pkg/client/listers/networking/v1"
	"github.com/oecp/rama/pkg/utils"
	"github.com/pkg/errors"
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

type Manager struct {
	ClusterName              string
	localClusterKubeClient   kubeclientset.Interface
	localClusterRamaClient   versioned.Interface
	remoteSubnetLister       listers.RemoteSubnetLister
	remoteSubnetSynced       cache.InformerSynced
	remoteVtepLister         listers.RemoteVtepLister
	remoteVtepSynced         cache.InformerSynced
	localClusterSubnetLister listers.SubnetLister
	localClusterSubnetSynced cache.InformerSynced

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

func NewRemoteClusterManager(
	localClusterKubeClient kubeclientset.Interface,
	localClusterRamaClient versioned.Interface,
	rc *networkingv1.RemoteCluster,
	remoteSubnetLister listers.RemoteSubnetLister,
	remoteSubnetHasSynced cache.InformerSynced,
	localClusterSubnetLister listers.SubnetLister,
	localClusterSubnetSynced cache.InformerSynced,
	remoteVtepInformer v1.RemoteVtepInformer) (*Manager, error) {
	defer func() {
		if err := recover(); err != nil {
			s, _ := jsoniter.MarshalToString(rc)
			klog.Errorf("Panic hanppened. Can't new remote cluster manager. Maybe wrong kube config. err=%v. remote cluster=%v", err, s)
		}
	}()

	config, err := utils.BuildClusterConfig(rc)
	if err != nil {
		return nil, err
	}
	config.Timeout = time.Duration(rc.Spec.ConnConfig.Timeout) * time.Second

	kubeClient := kubeclientset.NewForConfigOrDie(config)
	ramaClient := versioned.NewForConfigOrDie(restclient.AddUserAgent(config, UserAgentName))
	kubeInformerFactory := informers.NewSharedInformerFactory(kubeClient, 0)
	ramaInformerFactory := externalversions.NewSharedInformerFactory(ramaClient, 0)

	networkInformer := ramaInformerFactory.Networking().V1().Networks()
	nodeInformer := kubeInformerFactory.Core().V1().Nodes()
	subnetInformer := ramaInformerFactory.Networking().V1().RemoteSubnets()
	ipInformer := ramaInformerFactory.Networking().V1().IPInstances()

	if err := ipInformer.Informer().GetIndexer().AddIndexers(cache.Indexers{
		ByNodeNameIndexer: indexByNodeName,
	}); err != nil {
		klog.Error("index by node name failed. err=%v. ", err)
		return nil, errors.New("Can't add indexer")
	}

	rcManager := &Manager{
		ClusterName:              rc.Name,
		localClusterKubeClient:   localClusterKubeClient,
		localClusterRamaClient:   localClusterRamaClient,
		remoteSubnetLister:       remoteSubnetLister,
		remoteSubnetSynced:       remoteSubnetHasSynced,
		localClusterSubnetLister: localClusterSubnetLister,
		localClusterSubnetSynced: localClusterSubnetSynced,
		remoteVtepLister:         remoteVtepInformer.Lister(),
		remoteVtepSynced:         remoteVtepInformer.Informer().HasSynced,
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
		FilterFunc: rcManager.filterNode,
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    rcManager.addOrDelNode,
			UpdateFunc: rcManager.updateNode,
			DeleteFunc: rcManager.addOrDelNode,
		},
	})

	subnetInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: rcManager.filterSubnet,
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    rcManager.addOrDelSubnet,
			UpdateFunc: rcManager.updateSubnet,
			DeleteFunc: rcManager.addOrDelSubnet,
		},
	})

	ipInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: rcManager.filterIPInstance,
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    rcManager.addOrDelIPInstance,
			UpdateFunc: rcManager.updateIPInstance,
			DeleteFunc: rcManager.addOrDelIPInstance,
		},
	})
	rcManager.localClusterKubeClient = localClusterKubeClient
	return rcManager, nil
}

func indexByNodeName(obj interface{}) ([]string, error) {
	instance, ok := obj.(*networkingv1.IPInstance)
	if ok {
		return []string{instance.Status.NodeName}, nil
	}
	return []string{}, nil
}
