package remotecluster

import (
	"context"
	"fmt"
	"sync"
	"time"

	apiv1 "github.com/oecp/rama/pkg/apis/networking/v1"
	"github.com/oecp/rama/pkg/client/clientset/versioned"
	informers "github.com/oecp/rama/pkg/client/informers/externalversions/networking/v1"
	listers "github.com/oecp/rama/pkg/client/listers/networking/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtimeutil "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"
)

const (
	ControllerName = "remotecluster"

	ByRemoteClusterIDIndexer = "remoteclusterip"
	// HealthCheckPeriod Every HealthCheckPeriod will resync remote cluster cache and check rc
	// health. Default: 10 second. Set to zero will also use the default value
	HealthCheckPeriod = 10 * time.Second
)

type Controller struct {
	kubeClient           kubeclientset.Interface
	ramaClient           versioned.Interface
	remoteClusterLister  listers.RemoteClusterLister
	remoteClusterSynced  cache.InformerSynced
	remoteClusterIndexer cache.Indexer
	remoteClusterQueue   workqueue.RateLimitingInterface
	remoteClusterCache   Cache
	recorder             record.EventRecorder
	rcManagerQueue       workqueue.RateLimitingInterface
}

func NewController(
	recorder record.EventRecorder,
	kubeClient kubeclientset.Interface,
	ramaClient versioned.Interface,
	remoteClusterInformer informers.RemoteClusterInformer) *Controller {
	runtimeutil.Must(apiv1.AddToScheme(scheme.Scheme))

	if err := remoteClusterInformer.Informer().GetIndexer().AddIndexers(cache.Indexers{
		ByRemoteClusterIDIndexer: indexByRemoteClusterID,
	}); err != nil {
		klog.Fatalf("create remote cluster informer err. err=%v", err)
	}

	c := &Controller{
		remoteClusterCache: Cache{
			mu:               sync.RWMutex{},
			remoteClusterMap: make(map[uint32]*Manager),
		},
		kubeClient:           kubeClient,
		ramaClient:           ramaClient,
		remoteClusterLister:  remoteClusterInformer.Lister(),
		remoteClusterSynced:  remoteClusterInformer.Informer().HasSynced,
		remoteClusterIndexer: remoteClusterInformer.Informer().GetIndexer(),
		remoteClusterQueue:   workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), ControllerName),
		recorder:             recorder,
	}

	remoteClusterInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: c.filterRemoteCluster,
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    c.addRemoteCluster,
			UpdateFunc: c.updateRemoteCluster,
			DeleteFunc: c.deleteRemoteCluster,
		},
	})

	return c
}

func (c *Controller) Run(stopCh <-chan struct{}) error {
	defer runtimeutil.HandleCrash()
	defer c.rcManagerQueue.ShutDown()
	defer c.remoteClusterQueue.ShutDown()

	klog.Infof("Starting %s controller", ControllerName)

	klog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.remoteClusterSynced); !ok {
		return fmt.Errorf("%s failed to wait for caches to sync", ControllerName)
	}

	// start workers
	klog.Info("Starting workers")
	go wait.Until(c.runRemoteClusterWorker, time.Second, stopCh)
	go wait.Until(c.updateRemoteClusterStatus, HealthCheckPeriod, stopCh)
	go c.processRCManagerQueue(stopCh)
	<-stopCh

	klog.Info("Shutting down workers")
	return nil
}

func (c *Controller) delRemoteClusterClient(key uint32) error {
	c.remoteClusterCache.mu.Lock()
	defer c.remoteClusterCache.mu.Unlock()
	// todo any way to release client?
	delete(c.remoteClusterCache.remoteClusterMap, key)
	klog.Infof("Delete remote cluster ache. key=%v", key)
	return nil
}

// clusterID is not allowed to modify, webhook will ensure that
func (c *Controller) addOrUpdateRemoteClusterManager(key uint32, rc *apiv1.RemoteCluster) error {
	c.remoteClusterCache.mu.Lock()
	defer c.remoteClusterCache.mu.Unlock()

	client := c.remoteClusterCache.remoteClusterMap[key]
	if client != nil {
		// todo update logic
	}

	rcManager, err := NewRemoteClusterManager(c.kubeClient, rc)
	if err != nil || rcManager.ramaClient == nil || rcManager.kubeClient == nil {
		c.recorder.Eventf(rc, corev1.EventTypeWarning, "ErrClusterConnectionConfig", fmt.Sprintf("Can't connect to remote cluster %v", key))
		return nil
	}
	c.remoteClusterCache.remoteClusterMap[key] = rcManager
	c.rcManagerQueue.Add(key)
	return nil
}

// health checking and resync cache
func (c *Controller) updateRemoteClusterStatus() {
	remoteClusters, err := c.ramaClient.NetworkingV1().RemoteClusters().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		klog.Error(err)
		return
	}

	var wg sync.WaitGroup
	for _, obj := range remoteClusters.Items {
		manager, exists := c.remoteClusterCache.Get(obj.Spec.ClusterID)
		if !exists || manager == nil {
			// todo register
		}
		wg.Add(1)
		go c.updateSingleRCManager(manager, &wg)
	}
	wg.Wait()
}

func (c *Controller) updateSingleRCManager(rcClient *Manager, wg *sync.WaitGroup) {

}

func (c *Controller) processRCManagerQueue(stopCh <-chan struct{}) {
	for c.startRemoteClusterManager(stopCh) {
	}
}

func (c *Controller) startRemoteClusterManager(stopCh <-chan struct{}) bool {
	defer runtimeutil.HandleCrash()

	obj, shutdown := c.rcManagerQueue.Get()
	if shutdown {
		return false
	}
	clusterID, ok := obj.(uint32)
	if !ok {
		klog.Errorf("Can't convert obj in rc manager queue. obj=%v", obj)
		return true
	}

	rcManager, exists := c.remoteClusterCache.Get(clusterID)
	if !exists {
		klog.Error("Can't find rcManager. clusterID=%v", clusterID)
		return true
	}
	klog.Info("Start single remote cluster manager. clusterID=%v", clusterID)
	go func() {
		if ok := cache.WaitForCacheSync(stopCh, rcManager.nodeSynced, rcManager.subnetSynced, rcManager.ipSynced); !ok {
			klog.Errorf("failed to wait for remote cluster caches to sync. clusterID=%v", clusterID)
			return
		}
		go wait.Until(rcManager.runNodeWorker, 1*time.Second, stopCh)
		go wait.Until(rcManager.runNodeWorker, 1*time.Second, stopCh)
		go wait.Until(rcManager.runIPInstanceWorker, 1*time.Second, stopCh)
	}()
	go rcManager.kubeInformerFactory.Start(stopCh)
	go rcManager.ramaInformerFactory.Start(stopCh)

	return true

}
