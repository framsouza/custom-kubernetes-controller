package main

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	appsinformers "k8s.io/client-go/informers/apps/v1"
	"k8s.io/client-go/kubernetes"
	appslisters "k8s.io/client-go/listers/apps/v1"
	"k8s.io/client-go/tools/cache"
	workqueue "k8s.io/client-go/util/workqueue"
)

// defining controller struct
type controller struct {
	clientset         kubernetes.Interface         // Interacts with kubernetes resources
	deployLister      appslisters.DeploymentLister // Get the resources (Listers)
	deployCachedSyncd cache.InformerSynced         // Check if cache has been syncd
	queue             workqueue.RateLimitingInterface
}

// Read and populate controller fields
func NewController(clientset kubernetes.Interface, deployInformer appsinformers.DeploymentInformer) *controller {
	c := &controller{
		clientset:         clientset,
		deployLister:      deployInformer.Lister(),
		deployCachedSyncd: deployInformer.Informer().HasSynced,
		queue:             workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "queue"),
	}

	deployInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    c.handleAdd,
			DeleteFunc: c.handleDel,
		},
	)

	return c
}

// Make sure the cache is syncd,
func (c *controller) run(ch <-chan struct{}) {
	fmt.Print("Starting controller")
	if !cache.WaitForCacheSync(ch, c.deployCachedSyncd) {
		fmt.Print("waiting for cahce to be synced n")
	}

	// go routine, run this func until the channel is closed
	go wait.Until(c.worker, 1*time.Second, ch)
	<-ch
}

// continously run a func to collect the events from the queue
func (c *controller) worker() {
	for c.processItem() {

	}
}

// Collect the values from the queue
func (c *controller) processItem() bool {
	item, shutdown := c.queue.Get()
	if shutdown {
		return false
	}

	// Make sure the same obj won't be proceded again
	defer c.queue.Forget(item)

	// Get namespace
	key, err := cache.MetaNamespaceKeyFunc(item)
	if err != nil {
		fmt.Printf("getting key from cache %s\n", err.Error())
	}

	// return namespace/name
	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		fmt.Printf("Splitting key into namespace and name %s\n", err.Error())
		return false
	}

	// query the api to check if the object has been deleted from cluster
	ctx := context.Background()
	_, err = c.clientset.AppsV1().Deployments(ns).Get(ctx, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		fmt.Printf("Handling delete event for deployment %s\n", name)
		// delete service
		err = c.clientset.CoreV1().Services(ns).Delete(ctx, name, metav1.DeleteOptions{})
		if err != nil {
			fmt.Printf("deleteing service %s, error %s\n", name, err.Error())
			return false
		}
		// delete ingress
		err = c.clientset.NetworkingV1().Ingresses(ns).Delete(ctx, name, metav1.DeleteOptions{})
		if err != nil {
			fmt.Printf("deleteing ingress %s, error %s\n", name, err.Error())
			return false
		}

		return true
	}

	err = c.syncDeployment(ns, name)
	if err != nil {
		fmt.Printf("syncing deployment %s\n", err.Error())
		return false
	}

	return true
}

func (c *controller) syncDeployment(ns, name string) error {
	ctx := context.Background()

	deploy, err := c.deployLister.Deployments(ns).Get(name)
	if err != nil {
		fmt.Printf("getting deploytment from lister %s\n", err.Error())
	}

	if err != nil {
		panic(err.Error())
	}

	//create service
	// modify to figure out the port our deployment containr is listerning on
	svc := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploy.Name,
			Namespace: ns,
		},
		Spec: corev1.ServiceSpec{
			Selector: deployLabels(*deploy),
			Ports: []corev1.ServicePort{
				corev1.ServicePort{
					Name: "http",
					Port: 80,
				},
			},
		},
	}

	s, err := c.clientset.CoreV1().Services(ns).Create(ctx, &svc, metav1.CreateOptions{})
	if err != nil {
		fmt.Printf("creating service %s\n", err.Error())
	}

	return createIngress(ctx, c.clientset, s)
}

// creat ingress / make sure the ingress controller exists
func createIngress(ctx context.Context, client kubernetes.Interface, svc *corev1.Service) error {
	pathType := "Prefix"
	ingress := netv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      svc.Name,
			Namespace: svc.Namespace,
			Annotations: map[string]string{
				//"nginx.ingress.kubernetes.io/rewrite-target": "/",
				"kubernetes.io/ingress.class": "nginx",
			},
		},
		Spec: netv1.IngressSpec{
			Rules: []netv1.IngressRule{
				netv1.IngressRule{
					Host: "nginx.fs.co",
					IngressRuleValue: netv1.IngressRuleValue{
						HTTP: &netv1.HTTPIngressRuleValue{
							Paths: []netv1.HTTPIngressPath{
								netv1.HTTPIngressPath{
									Path: "/",
									//Path:     fmt.Sprintf("/%s", svc.Name),
									PathType: (*netv1.PathType)(&pathType),
									Backend: netv1.IngressBackend{
										Service: &netv1.IngressServiceBackend{
											Name: svc.Name,
											Port: netv1.ServiceBackendPort{
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
		},
	}
	_, err := client.NetworkingV1().Ingresses(svc.Namespace).Create(ctx, &ingress, metav1.CreateOptions{})
	if err != nil {
		fmt.Printf("creating ingress, %s\n", err.Error())
	}
	return err

}

// Return the deployment labels
func deployLabels(deploy appsv1.Deployment) map[string]string {
	return deploy.Spec.Template.Labels
}

// add the object in the worker queue
func (c *controller) handleAdd(obj interface{}) {
	fmt.Print("Adding deployment\n")
	c.queue.Add(obj)
}

// delete object from the worker queue
func (c *controller) handleDel(obj interface{}) {
	fmt.Print("Deleting deployment\n")
	c.queue.Add(obj)

}
