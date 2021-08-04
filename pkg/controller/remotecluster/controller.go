package remotecluster

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	apiv1 "github.com/oecp/rama/pkg/apis/networking/v1"
	"github.com/oecp/rama/pkg/client/clientset/versioned"
	"github.com/oecp/rama/pkg/client/informers/externalversions"
	informers "github.com/oecp/rama/pkg/client/informers/externalversions/networking/v1"
	listers "github.com/oecp/rama/pkg/client/listers/networking/v1"
	"github.com/oecp/rama/pkg/rcmanager"
	"github.com/oecp/rama/pkg/utils"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	runtimeutil "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	coreinformers "k8s.io/client-go/informers"
	kubeclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"
)

const (
	ControllerName = "remotecluster"

	// HealthCheckPeriod Every HealthCheckPeriod will resync remote cluster cache and check rc
	// health. Default: 20 second. Set to zero will also use the default value
	HealthCheckPeriod = 20 * time.Second
)

type Controller struct {
	kubeClient          kubeclientset.Interface
	ramaClient          versioned.Interface
	kubeInformerFactory coreinformers.SharedInformerFactory
	RamaInformerFactory externalversions.SharedInformerFactory

	remoteClusterLister      listers.RemoteClusterLister
	remoteClusterSynced      cache.InformerSynced
	remoteClusterQueue       workqueue.RateLimitingInterface
	remoteSubnetLister       listers.RemoteSubnetLister
	remoteSubnetSynced       cache.InformerSynced
	remoteVtepInformer       informers.RemoteVtepInformer
	localClusterSubnetLister listers.SubnetLister
	localClusterSubnetSynced cache.InformerSynced

	remoteClusterManagerCache Cache
	recorder                  record.EventRecorder
	rcManagerQueue            workqueue.RateLimitingInterface
}

func NewController(
	kubeClient kubeclientset.Interface,
	ramaClient versioned.Interface,
	remoteClusterInformer informers.RemoteClusterInformer,
	remoteSubnetInformer informers.RemoteSubnetInformer,
	localClusterSubnetInformer informers.SubnetInformer,
	remoteVtepInformer informers.RemoteVtepInformer) *Controller {
	runtimeutil.Must(apiv1.AddToScheme(scheme.Scheme))

	klog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: ControllerName})

	c := &Controller{
		remoteClusterManagerCache: Cache{
			remoteClusterMap: make(map[string]*rcmanager.Manager),
		},
		kubeClient:               kubeClient,
		ramaClient:               ramaClient,
		remoteClusterLister:      remoteClusterInformer.Lister(),
		remoteClusterSynced:      remoteClusterInformer.Informer().HasSynced,
		remoteSubnetLister:       remoteSubnetInformer.Lister(),
		remoteSubnetSynced:       remoteSubnetInformer.Informer().HasSynced,
		localClusterSubnetLister: localClusterSubnetInformer.Lister(),
		localClusterSubnetSynced: localClusterSubnetInformer.Informer().HasSynced,
		remoteVtepInformer:       remoteVtepInformer,
		remoteClusterQueue:       workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), ControllerName),
		rcManagerQueue:           workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "remoteclustermanager"),
		recorder:                 recorder,
	}

	remoteClusterInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: c.filterRemoteCluster,
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    c.addOrDelRemoteCluster,
			UpdateFunc: c.updateRemoteCluster,
			DeleteFunc: c.addOrDelRemoteCluster,
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
	if ok := cache.WaitForCacheSync(stopCh, c.remoteClusterSynced, c.remoteSubnetSynced, c.localClusterSubnetSynced); !ok {
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

// health checking and resync cache. remote clsuter garbage cache will also be deleted
func (c *Controller) updateRemoteClusterStatus() {
	remoteClusters, err := c.remoteClusterLister.List(labels.NewSelector())
	if err != nil {
		klog.Errorf("Can't list remote cluster. err=%v", err)
		return
	}

	var wg sync.WaitGroup
	for _, obj := range remoteClusters {
		manager, exists := c.remoteClusterManagerCache.Get(obj.Name)
		if !exists {
			if err = c.addOrUpdateRemoteClusterManager(obj); err != nil {
				continue
			}
		}
		wg.Add(1)
		go c.healCheck(manager, &wg)
	}
	wg.Wait()
	klog.Info("Update Remote Cluster Status Finished.")
}

// use remove+add instead of update
func (c *Controller) addOrUpdateRemoteClusterManager(rc *apiv1.RemoteCluster) error {
	clusterName := rc.Name
	_, exist := c.remoteClusterManagerCache.Get(clusterName)
	if exist {
		c.delRemoteClusterManager(clusterName)
	}

	rcManager, err := rcmanager.NewRemoteClusterManager(c.kubeClient, c.ramaClient, rc, c.remoteSubnetLister, c.remoteSubnetSynced,
		c.localClusterSubnetLister, c.localClusterSubnetSynced, c.remoteVtepInformer)
	if err != nil || rcManager.RamaClient == nil || rcManager.KubeClient == nil {
		c.recorder.Eventf(rc, corev1.EventTypeWarning, "ErrClusterConnectionConfig", fmt.Sprintf("Can't connect to remote cluster %v", clusterName))
		return errors.Errorf("Can't connect to remote cluster %v", clusterName)
	}

	//err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
	//	clusterIDStr := strconv.FormatInt(int64(clusterName), 10)
	//	rcs, err := c.remoteClusterIndexer.ByIndex(ByRemoteClusterIDIndexer, clusterIDStr)
	//	if err != nil {
	//		return err
	//	}
	//
	//})
	//if err != nil {
	//	klog.Errorf()
	//	return errors.Newf()
	//}
	c.remoteClusterManagerCache.Set(clusterName, rcManager)
	c.rcManagerQueue.Add(clusterName)
	return nil
}

func (c *Controller) healCheck(manager *rcmanager.Manager, wg *sync.WaitGroup) {
	// todo metrics
	defer func() {
		if err := recover(); err != nil {
			klog.Errorf("healCheck panic. err=%v", err)
		}
	}()
	defer wg.Done()

	conditions := apiv1.RemoteClusterStatus{Conditions: make([]apiv1.ClusterCondition, 0)}

	body, err := manager.KubeClient.DiscoveryClient.RESTClient().Get().AbsPath("/healthz").Do(context.TODO()).Raw()
	if err != nil {
		runtimeutil.HandleError(errors.Wrapf(err, "Cluster Health Check failed for cluster %v", manager.ClusterName))
		conditions.Conditions = append(conditions.Conditions, utils.NewClusterOffline(err))
	} else {
		if !strings.EqualFold(string(body), "ok") {
			conditions.Conditions = append(conditions.Conditions, utils.NewClusterNotReady(err), utils.NewClusterNotOffline())
		} else {
			conditions.Conditions = append(conditions.Conditions, utils.NewClusterReady())
		}
	}

	return
}

func (c *Controller) delRemoteClusterManager(clusterName string) {
	c.remoteClusterManagerCache.Del(clusterName)
	klog.Infof("Delete remote cluster cache. key=%v", clusterName)
}

func (c *Controller) processRCManagerQueue(stopCh <-chan struct{}) {
	for c.startRemoteClusterManager(stopCh) {
	}
}

func (c *Controller) startRemoteClusterManager(stopCh <-chan struct{}) bool {
	defer func() {
		if err := recover(); err != nil {
			klog.Errorf("startRemoteClusterManager panic. err=%v", err)
		}
	}()

	obj, shutdown := c.rcManagerQueue.Get()
	if shutdown {
		return false
	}
	clusterName, ok := obj.(string)
	if !ok {
		klog.Errorf("Can't convert obj in rc manager queue. obj=%v", obj)
		return true
	}

	rcManager, exists := c.remoteClusterManagerCache.Get(clusterName)
	if !exists {
		klog.Errorf("Can't find rcManager. clusterName=%v", clusterName)
		return true
	}
	klog.Infof("Start single remote cluster manager. clusterName=%v", clusterName)
	go func() {
		if ok := cache.WaitForCacheSync(stopCh, rcManager.NodeSynced, rcManager.SubnetSynced, rcManager.IPSynced); !ok {
			klog.Errorf("failed to wait for remote cluster caches to sync. clusterName=%v", clusterName)
			return
		}
		go wait.Until(rcManager.RunNodeWorker, 1*time.Second, stopCh)
		go wait.Until(rcManager.RunSubnetWorker, 1*time.Second, stopCh)
		go wait.Until(rcManager.RunIPInstanceWorker, 1*time.Second, stopCh)
	}()
	go rcManager.KubeInformerFactory.Start(stopCh)
	go rcManager.RamaInformerFactory.Start(stopCh)
	return true
}
