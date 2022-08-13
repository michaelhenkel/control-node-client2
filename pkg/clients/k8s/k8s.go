package k8s

import (
	"fmt"
	"path/filepath"
	"time"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	api "github.com/michaelhenkel/control-node-client2/pkg/schema"
	corev1 "k8s.io/api/core/v1"
	contrailClient "ssd-git.juniper.net/contrail/cn2/contrail/pkg/client/clientset_generated/clientset"
)

const (
	Closed   = 0
	Added    = 1
	Modified = 2
	Deleted  = 3
	Error    = -1
)

type Client struct {
	KubernetesClientSet *kubernetes.Clientset
	ContrailClientSet   *contrailClient.Clientset
	dynamicClientSet    dynamic.Interface
}

func New(kubeConfigPath string) (*Client, error) {
	config, err := get_config(kubeConfigPath)
	if err != nil {
		return nil, err
	}
	contrailClientSet, kubernetesClientSet, dynamicClientSet, _ := getClientSets(config)
	return &Client{
		KubernetesClientSet: kubernetesClientSet,
		ContrailClientSet:   contrailClientSet,
		dynamicClientSet:    dynamicClientSet,
	}, nil
}

func (c *Client) Watch(serviceImportChan chan api.ServiceCommunity, serviceExportChan chan api.ServiceCommunity) error {
	stopCh := make(chan struct{})
	sifList, _ := NewSharedInformerFactory(c.KubernetesClientSet, c.ContrailClientSet, c.dynamicClientSet, serviceImportChan, serviceExportChan)

	for _, sif := range sifList {
		sif.Start(stopCh)
		sif.WaitForCacheSync(stopCh)
	}
	return nil
}

func get_config(kubeConfigPath string) (*rest.Config, error) {
	var err error
	var kconfig string
	config, _ := rest.InClusterConfig()
	if config == nil {
		if kubeConfigPath != "" {
			kconfig = kubeConfigPath
		} else if home := homedir.HomeDir(); home != "" {
			kconfig = filepath.Join(home, ".kube", "config")
		}
		config, err = clientcmd.BuildConfigFromFlags("", kconfig)
		if err != nil {
			return nil, err
		}
	}

	return config, nil
}

func getClientSets(config *rest.Config) (*contrailClient.Clientset, *kubernetes.Clientset, dynamic.Interface, error) {

	contrailClientSet, err := contrailClient.NewForConfig(config)
	if err != nil {
		return nil, nil, nil, err
	}
	kubernetesClientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, nil, err
	}

	dynamicClientSet, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, nil, nil, err
	}
	return contrailClientSet, kubernetesClientSet, dynamicClientSet, nil
}

func NewSharedInformerFactory(kubernetesClientSet *kubernetes.Clientset, contrailClientSet *contrailClient.Clientset, dynamicClientSet dynamic.Interface, serviceImportChan chan api.ServiceCommunity, serviceExportChan chan api.ServiceCommunity) ([]dynamicinformer.DynamicSharedInformerFactory, error) {
	var gvrList []schema.GroupVersionResource
	gvrList = append(gvrList, schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "services",
	})
	/*
		contrailResources, err := contrailClientSet.DiscoveryClient.ServerResourcesForGroupVersion("core.contrail.juniper.net/v1")
		if err != nil {
			return nil, err
		}

		for _, contrailResource := range contrailResources.APIResources {
			resourceNameList := strings.Split(contrailResource.Name, "/status")
			gvrList = append(gvrList, schema.GroupVersionResource{
				Group:    "core.contrail.juniper.net",
				Version:  "v1",
				Resource: resourceNameList[0],
			})
		}
	*/
	var sifList []dynamicinformer.DynamicSharedInformerFactory
	for _, gvr := range gvrList {
		sif := dynamicinformer.NewDynamicSharedInformerFactory(dynamicClientSet, time.Minute*10)
		dynamicInformer := sif.ForResource(gvr)
		dynamicInformer.Informer().AddEventHandler(resourceEventHandler(&watchHandlerFunc{serviceImportChan, serviceExportChan, contrailClientSet, kubernetesClientSet}))
		sifList = append(sifList, sif)
	}
	return sifList, nil
}

type watchHandlerFunc struct {
	serviceImportChan   chan api.ServiceCommunity
	serviceExportChan   chan api.ServiceCommunity
	contrailClientSet   *contrailClient.Clientset
	kubernetesClientSet *kubernetes.Clientset
}

func (h *watchHandlerFunc) HandleEvent(event api.Action, obj *unstructured.Unstructured) error {

	kind, _, err := unstructured.NestedString(obj.Object, "kind")
	if err != nil {
		fmt.Println(err)
	}
	switch kind {
	case "Service":
		service := &corev1.Service{}
		runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), service)
		handleService(service, event, h.serviceImportChan, h.serviceExportChan, h.contrailClientSet, h.kubernetesClientSet)
	default:
		fmt.Println("not found")
	}
	return nil
}

type WatchEventHandler interface {
	HandleEvent(event api.Action, obj *unstructured.Unstructured) error
}

func resourceEventHandler(handler WatchEventHandler) cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {

			u := obj.(*unstructured.Unstructured)
			/*
				name, namefound, err := unstructured.NestedString(u.Object, "metadata", "name")
				if err != nil {
					fmt.Println(err)
				}


					rv, rvfound, err := unstructured.NestedString(u.Object, "metadata", "resourceVersion")
					if err != nil {
						fmt.Println(err)
					}

					kind, kindfound, err := unstructured.NestedString(u.Object, "kind")
					if err != nil {
						fmt.Println(err)
					}
					if namefound && kindfound && rvfound {
						fmt.Println("add", kind, name, rv)
					}
			*/
			handler.HandleEvent(api.Add, u)

		},
		UpdateFunc: func(oldObj interface{}, newObj interface{}) {
			//uold := oldObj.(*unstructured.Unstructured)
			u := newObj.(*unstructured.Unstructured)
			/*
				name, namefound, err := unstructured.NestedString(u.Object, "metadata", "name")
				if err != nil {
					fmt.Println(err)
				}

				rv, rvfound, err := unstructured.NestedString(u.Object, "metadata", "resourceVersion")
				if err != nil {
					fmt.Println(err)
				}

				oldrv, oldrvfound, err := unstructured.NestedString(uold.Object, "metadata", "resourceVersion")
				if err != nil {
					fmt.Println(err)
				}

				kind, kindfound, err := unstructured.NestedString(u.Object, "kind")
				if err != nil {
					fmt.Println(err)
				}
				if namefound && kindfound && rvfound && oldrvfound {
					//fmt.Println("update", kind, name, rv, oldrv)
				}
			*/
			handler.HandleEvent(api.Update, u)

		},
		DeleteFunc: func(obj interface{}) {
			u := obj.(*unstructured.Unstructured)
			handler.HandleEvent(api.Del, u)

		},
	}
}
