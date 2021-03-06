package pkg

import (
	"context"
	v13 "k8s.io/api/core/v1"
	v15 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v16 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	informer "k8s.io/client-go/informers/core/v1"
	netInformer "k8s.io/client-go/informers/networking/v1"
	"k8s.io/client-go/kubernetes"
	v12 "k8s.io/client-go/listers/core/v1"
	v1 "k8s.io/client-go/listers/networking/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"reflect"
	"time"
)

const (
	workNum = 5
	maxRetry = 10
)

type controller struct {
	client kubernetes.Interface
	ingressLister v1.IngressLister
	servicelister v12.ServiceLister
	queue workqueue.RateLimitingInterface
}

func (c *controller) updataService(oldObj interface{}, newObj interface{}) {
	println("update Service")
	// todo compare annotation
	if reflect.DeepEqual(oldObj, newObj) {
		return
	}
	c.enqueue(newObj)
}

func (c *controller) addService(obj interface{}) {
	println("add Service")
	c.enqueue(obj)
}

func (c *controller) enqueue(obj interface{})  {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
	}

	c.queue.Add(key)
}

func (c *controller) deleteIngress(obj interface{}) {
	println("delete ingress")
	ingress := obj.(*v15.Ingress)
	ownerReference := v16.GetControllerOf(ingress)
	if ownerReference == nil {
		return
	}
	if ownerReference.Kind != "Service" {
		return
	}

	c.queue.Add(ingress.Namespace + "/" + ingress.Name)
}

func (c *controller ) Run(stopCh chan struct{}) {
	for i := 0; i < workNum; i++ {
		go wait.Until(c.worker, time.Minute, stopCh)
	}
	<- stopCh
}

func (c *controller) worker() {
	for c.processNextItem(){

	}
}

func (c *controller) processNextItem() bool {
	item, shutdown := c.queue.Get()
	if shutdown{
		return false
	}
	defer c.queue.Done(item)

	key := item.(string)

	err := c.syncService(key)
	if err != nil {
		c.handlerError(key, err)
	}
	return true
}

func (c *controller) syncService(key string) error {
	namespaceKey, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	// Delete
	service, err := c.servicelister.Services(namespaceKey).Get(name)
	if errors.IsNotFound(err) {
		return nil
	}
	if err != nil{
		return err
	}

	// Add and Delete
	_, ok := service.GetAnnotations()["ingress/http"]
	ingress, err := c.ingressLister.Ingresses(namespaceKey).Get(name)
	if err != nil && !errors.IsNotFound(err){
		return err
	}

	if ok && errors.IsNotFound(err){
		// create ingress
		ig := c.constructIngress(service)
		_, err := c.client.NetworkingV1().Ingresses(namespaceKey).Create(context.TODO(), ig, v16.CreateOptions{})
		if err != nil{
			return err
		}
	}else if !ok && ingress != nil {
		// delete ingress
		err := c.client.NetworkingV1().Ingresses(namespaceKey).Delete(context.TODO(), name, v16.DeleteOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *controller) handlerError(key string, err error) {
	if c.queue.NumRequeues(key) <= maxRetry {
		c.queue.AddRateLimited(key)
		return
	}

	runtime.HandleError(err)
	c.queue.Forget(key)
}

func (c *controller) constructIngress(service *v13.Service) *v15.Ingress {
	ingress := v15.Ingress{}

	ingress.ObjectMeta.OwnerReferences = []v16.OwnerReference{
		*v16.NewControllerRef(service, v13.SchemeGroupVersion.WithKind("service")),
	}

	ingress.Name = service.Name
	ingress.Namespace = service.Namespace
	PathType := v15.PathTypePrefix
	icn := "nginx"
	ingress.Spec = v15.IngressSpec{
		IngressClassName: &icn,
		Rules: []v15.IngressRule{
			{
				Host: "example.com",
				IngressRuleValue: v15.IngressRuleValue{
					HTTP: &v15.HTTPIngressRuleValue{
						Paths: []v15.HTTPIngressPath{
							{
								Path: "/",
								PathType: &PathType,
								Backend: v15.IngressBackend{
									Service: &v15.IngressServiceBackend{
										Name: service.Name,
										Port: v15.ServiceBackendPort{
											Number: 80,
										},
									},
								},

							},
						},
					},
				},

			},
		},
	}

	return &ingress
}

func NewController(client kubernetes.Interface, serviceInformer informer.ServiceInformer, ingressInformer netInformer.IngressInformer) controller {
	c := controller{
		client:        client,
		ingressLister: ingressInformer.Lister(),
		servicelister: serviceInformer.Lister(),
		queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "ingressManager"),
	}
	serviceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: c.addService,
		UpdateFunc: c.updataService,
	})

	ingressInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		DeleteFunc: c.deleteIngress,
	})
	return c
}