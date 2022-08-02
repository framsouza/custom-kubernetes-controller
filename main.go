package main

import (
	"flag"
	"time"

	//"github.com/docker/cli/cli/command/stack/kubernetes"

	//"github.com/docker/cli/kubernetes/client/informers"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	var kubeconfig *string
	kubeconfig, err := rest.InClusterConfig()

	/*if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	*/
	flag.Parse()

	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	// create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	//create a channel
	ch := make(chan struct{})

	informer := informers.NewSharedInformerFactory(clientset, 10*time.Minute)
	c := NewController(clientset, informer.Apps().V1().Deployments())
	// Start the informer
	informer.Start(ch)
	c.run(ch)

}
