package remotecluster

import (
	"context"
	"fmt"
	"runtime"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"

	jsoniter "github.com/json-iterator/go"
	"github.com/stretchr/testify/assert"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	clientconfig "sigs.k8s.io/controller-runtime/pkg/client/config"
)

func TestWatchRemoteCluster(t *testing.T) {
	config, err := clientconfig.GetConfig()
	assert.Nil(t, err)
	kubeClient := kubernetes.NewForConfigOrDie(config)

	informerFactory := informers.NewSharedInformerFactory(kubeClient, 0)
	secretInformer := informerFactory.Core().V1().Secrets()
	secretLister := informerFactory.Core().V1().Secrets().Lister()
	queue := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "secret")

	addFunc := func(obj interface{}) {
		t.Log("add func")
		secret, ok := obj.(*apiv1.Secret)
		if !ok {
			t.Fatal("Convert not ok")
		}
		s, err := jsoniter.MarshalToString(secret)
		t.Log(s, err)
		queue.Add(secret.Name)
	}
	updateFunc := func(oldObj, newObj interface{}) {
		t.Log("update func")
		secret, ok := newObj.(*apiv1.Secret)
		if !ok {
			t.Fatal("Convert not ok")
		}
		s, err := jsoniter.MarshalToString(secret)
		t.Log(s, err)
		queue.Add(secret.Name)
	}

	process := func() bool {
		t.Log("before queue getting")
		obj, shutdown := queue.Get()
		if shutdown {
			return false
		}
		s, _ := obj.(string)
		secret, err := secretLister.Secrets("default").Get(s)
		t.Log(secret)
		t.Log(err)
		return true
	}

	run := func() {
		t.Log("running")
		for process() {
			t.Log("process once")
		}
	}
	secretInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    addFunc,
		UpdateFunc: updateFunc,
		DeleteFunc: addFunc,
	})
	ch := make(chan struct{})
	now := time.Now()
	t.Log("start sync")
	t.Log(secretInformer.Informer().HasSynced())
	informerFactory.Start(ch)
	if ok := cache.WaitForCacheSync(ch, secretInformer.Informer().HasSynced); !ok {
		t.Error("failed to wait for caches to sync")
	}
	t.Logf("sync success. spent: %v", time.Since(now))

	go wait.Until(run, 1*time.Second, ch)
	<-ch
}

func TestNilSecret(t *testing.T) {
	config, err := clientconfig.GetConfig()
	assert.Nil(t, err)
	kubeClient := kubernetes.NewForConfigOrDie(config)
	infFac := informers.NewSharedInformerFactory(kubeClient, 0)
	lister := infFac.Core().V1().Secrets().Lister().Secrets("default")
	secret, err := lister.Get("notexist")
	t.Log(err)
	t.Log(secret)
}

func TestWaitUntil(t *testing.T) {
	ch := make(chan struct{})
	wait.Until(printHelloWorld, 1*time.Second, ch)
}

func printHelloWorld() {
	time.Sleep(2 * time.Second)
	fmt.Println("hello world", time.Now())
}

func TestWaitUtil2(t *testing.T) {
	ch := make(chan struct{})
	go wait.Until(printLong, 1*time.Second, ch)
	go wait.Until(goroutine, 1*time.Second, ch)
	go wait.Until(forloop, 1*time.Second, ch)
	<-ch
}

func printLong() {
	time.Sleep(100 * time.Second)
	fmt.Println("hello world", time.Now())
}

func goroutine() {
	fmt.Println("goroutine num:", runtime.NumGoroutine())
}

func forloop() {
	for {
		fmt.Println("forloop")
	}
}

func TestWaitUtil3(t *testing.T) {
	ch := make(chan struct{})
	go wait.Until(father, 1*time.Second, ch)
	time.Sleep(2 * time.Second)
	close(ch)
	fmt.Println("father over")
	time.Sleep(10 * time.Second)
}

func father() {
	i := 0
	for ; i < 10; i++ {
		i := i
		go func() {
			for {
				time.Sleep(1 * time.Second)
				fmt.Println("hello from ", i)
			}
		}()
	}
}

func TestErrKubeI(t *testing.T) {
	config, err := clientconfig.GetConfig()
	t.Log(err)
	kubeClient := kubernetes.NewForConfigOrDie(config)
	body, err := kubeClient.DiscoveryClient.RESTClient().Get().AbsPath("/healthz").Do(context.TODO()).Raw()
	t.Log(err)
	t.Log(string(body))
}
