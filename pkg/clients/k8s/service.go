package k8s

import (
	"context"
	"strings"

	api "github.com/michaelhenkel/control-node-client2/pkg/schema"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	contrailClient "ssd-git.juniper.net/contrail/cn2/contrail/pkg/client/clientset_generated/clientset"
)

func handleService(service *corev1.Service, event Event, serviceImportChan chan api.ServiceCommunity, serviceExportChan chan api.ServiceCommunity, contrailClientSet *contrailClient.Clientset, kubernetesClientSet *kubernetes.Clientset) error {
	var namespace, name string
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
	importValue, importOk := service.Annotations["service.contrail.juniper.net/import"]
	if importOk {

		if namespace != "" && name != "" {
			serviceImportChan <- api.ServiceCommunity{
				VirtualNetworkName:      name,
				VirtualNetworkNamespace: namespace,
				Community:               importValue,
			}
		}
	}
	exportValue, exportOk := service.Annotations["service.contrail.juniper.net/export"]
	if exportOk {
		if namespace != "" && name != "" {
			serviceExportChan <- api.ServiceCommunity{
				VirtualNetworkName:      name,
				VirtualNetworkNamespace: namespace,
				Community:               exportValue,
				ServiceName:             service.Name,
				ServiceNamespace:        service.Namespace,
			}
		}
	}
	if !importOk && !exportOk {
		rp, err := contrailClientSet.CoreV1().RoutingPolicies(namespace).Get(context.Background(), service.Name, metav1.GetOptions{})
		if err != nil {
			if !errors.IsNotFound(err) {
				return err
			}
		} else {
			for _, riRef := range rp.Spec.RoutingInstanceReferences {
				vn, err := contrailClientSet.CoreV1().VirtualNetworks(riRef.Namespace).Get(context.Background(), riRef.Name, metav1.GetOptions{})
				if err != nil {
					if !errors.IsNotFound(err) {
						return err
					}
				} else {
					rpRefs := vn.Spec.RoutingPolicyReferences
					var rpRefIdx int
					for idx, rpRef := range rpRefs {
						if rpRef.Name == rp.Name && rpRef.Namespace == rp.Namespace {
							rpRefIdx = idx
						}
					}
					if rpRefIdx != 0 {
						rpRefs[rpRefIdx] = rpRefs[len(rpRefs)-1]
						rpRefs = rpRefs[:len(rpRefs)-1]
					}
					vn.Spec.RoutingPolicyReferences = rpRefs
					_, err := contrailClientSet.CoreV1().VirtualNetworks(vn.Namespace).Update(context.Background(), vn, metav1.UpdateOptions{})
					if err != nil {
						return err
					}
				}
			}
			if err := contrailClientSet.CoreV1().RoutingPolicies(rp.Namespace).Delete(context.Background(), rp.Name, metav1.DeleteOptions{}); err != nil {
				return err
			}
		}
	}
	return nil

}
