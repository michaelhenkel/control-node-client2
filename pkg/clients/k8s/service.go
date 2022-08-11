package k8s

import (
	"context"
	"strings"

	api "github.com/michaelhenkel/control-node-client2/pkg/schema"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	contrailClient "ssd-git.juniper.net/contrail/cn2/contrail/pkg/client/clientset_generated/clientset"
)

func handleService(service *corev1.Service, callBackChan chan api.VirtualNetworkCommunity, contrailClientSet *contrailClient.Clientset) error {
	var namespace, name string
	importValue, ok := service.Annotations["service.contrail.juniper.net/import"]
	if ok {
		customVN, ok := service.Annotations["service.contrail.juniper.net/externalNetwork"]
		if ok {
			vnSlice := strings.Split(customVN, "/")
			if len(vnSlice) > 1 {
				namespace = vnSlice[0]
				name = vnSlice[1]
			} else {
				namespace = service.GetNamespace()
				name = vnSlice[0]
			}
		} else {
			vnList, err := contrailClientSet.CoreV1().VirtualNetworks("").List(context.Background(), metav1.ListOptions{})
			if err != nil {
				return err
			}
			for _, vn := range vnList.Items {
				if vn.Name == "default-servicenetwork" {
					name = vn.Name
					namespace = vn.Namespace
				}
			}
		}
		if namespace != "" && name != "" {
			callBackChan <- api.VirtualNetworkCommunity{
				Name:      name,
				Namespace: namespace,
				Community: importValue,
			}
		}
	}
	return nil

}
