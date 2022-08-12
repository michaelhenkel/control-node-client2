package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/michaelhenkel/control-node-client2/pkg/clients/control"
	"github.com/michaelhenkel/control-node-client2/pkg/clients/k8s"

	api "github.com/michaelhenkel/control-node-client2/pkg/schema"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ccapi "ssd-git.juniper.net/contrail/cn2/contrail/pkg/apis/core/v1"
)

var msg2 = `<stream:stream from="worker-3" to="network-control@contrailsystems.com" version="1.0" xml:lang="en" xmlns="" xmlns:stream="http://etherx.jabber.org/streams"  >`

func main() {

	stopChan := make(chan bool)
	var controlCallBackChan = make(chan api.PrefixCommunity)
	controlClient := control.NewClient("10.233.65.14:5269", controlCallBackChan)
	controlClient.Write(msg2)
	k8sClient, err := k8s.New("/home/mhenkel/admin.conf")
	if err != nil {
		panic(err)
	}
	var serviceImportChan = make(chan api.ServiceCommunity)
	var serviceExportChan = make(chan api.ServiceCommunity)

	var subscriberMap = make(map[string]bool)

	go func() {
		fmt.Println("starting service import watch")
		for serviceCommunity := range serviceImportChan {
			nameNamespace := fmt.Sprintf("%s/%s", serviceCommunity.VirtualNetworkNamespace, serviceCommunity.VirtualNetworkNamespace)
			if _, ok := subscriberMap[nameNamespace]; !ok {
				subscriberMap[nameNamespace] = true
				msg := subscriptionMessage(serviceCommunity.VirtualNetworkName, serviceCommunity.VirtualNetworkNamespace)
				controlClient.Write(msg)
			}
		}
	}()

	go func() {
		fmt.Println("starting service export watch")
		for serviceCommunity := range serviceExportChan {
			fmt.Println("received serviceCommunity", serviceCommunity)

			rp, err := serviceToRP(serviceCommunity, k8sClient)
			if err != nil {
				fmt.Println("cannot convert ep to rp", err)
			}
			if err := reconcileRP(rp, serviceCommunity, k8sClient); err != nil {
				fmt.Println("cannot reconcile rp", err)
			}

		}
	}()

	go func() {
		fmt.Println("starting community watch")
		for prefixCommunity := range controlCallBackChan {
			fmt.Println("received prefixCommunity", prefixCommunity)
			if err := rpToServiceEndpoint(prefixCommunity, k8sClient); err != nil {
				fmt.Println("cannot create service/endpoint", err)
			}
		}
	}()

	go k8sClient.Watch(serviceImportChan, serviceExportChan)
	go controlClient.Watch()

	<-stopChan
}

func rpToServiceEndpoint(prefixCommunity api.PrefixCommunity, k8sClient *k8s.Client) error {
	svcList, err := k8sClient.KubernetesClientSet.CoreV1().Services("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return err
	}
	port, communityId, protocol := decodePortProtocolCommunity(prefixCommunity.Community)
	var servicePrefixes = make(map[string]bool)
	var endpointPrefixes = make(map[string]bool)
	var servicePortProtocol = make(map[struct {
		port     int32
		protocol string
	}]bool)
	var endpointPortProtocol = make(map[struct {
		port     int32
		protocol string
	}]bool)

	for _, svc := range svcList.Items {
		if id, ok := svc.Annotations["service.contrail.juniper.net/import"]; ok {
			if id == strconv.Itoa(int(communityId)) {
				for _, externalAddress := range svc.Spec.ExternalIPs {
					servicePrefixes[externalAddress] = true
				}
				for _, svcPort := range svc.Spec.Ports {
					if svcPort.Port == int32(port) && svcPort.Protocol == corev1.Protocol(protocol) {
						servicePortProtocol[struct {
							port     int32
							protocol string
						}{port: int32(port), protocol: protocol}] = true
					}
				}
				ep, err := k8sClient.KubernetesClientSet.CoreV1().Endpoints(svc.Namespace).Get(context.Background(), svc.Name, metav1.GetOptions{})
				if err != nil {
					if !errors.IsNotFound(err) {
						return err
					} else {
						ep, err = k8sClient.KubernetesClientSet.CoreV1().Endpoints(svc.Namespace).Create(context.Background(), &corev1.Endpoints{
							ObjectMeta: metav1.ObjectMeta{
								Name:      svc.Name,
								Namespace: svc.Namespace,
							},
						}, metav1.CreateOptions{})
						if err != nil {
							return err
						}
					}
				} else {
					for _, subset := range ep.Subsets {
						for _, address := range subset.Addresses {
							endpointPrefixes[address.IP] = true
						}
						for _, epPort := range subset.Ports {
							if epPort.Port == int32(port) && epPort.Protocol == corev1.Protocol(protocol) {
								endpointPortProtocol[struct {
									port     int32
									protocol string
								}{port: int32(port), protocol: protocol}] = true
							}
						}
					}

				}
				prefixList := strings.Split(prefixCommunity.Prefix, "/")

				updateService := false
				updateEndpoint := false

				if ok := servicePrefixes[prefixList[0]]; !ok {
					svc.Spec.ExternalIPs = append(svc.Spec.ExternalIPs, prefixList[0])
					updateService = true
				}
				if ok := endpointPrefixes[prefixList[0]]; !ok {
					if len(ep.Subsets) > 0 {
						ep.Subsets[0].Addresses = append(ep.Subsets[0].Addresses, corev1.EndpointAddress{
							IP: prefixList[0],
						})
					} else {
						ep.Subsets = []corev1.EndpointSubset{{
							Addresses: []corev1.EndpointAddress{{
								IP: prefixList[0],
							}},
						}}
					}
					updateEndpoint = true
				}

				portProtoStruct := struct {
					port     int32
					protocol string
				}{port: port, protocol: protocol}

				communityIdName := strconv.Itoa(int(communityId))
				if ok := servicePortProtocol[portProtoStruct]; !ok {
					svc.Spec.Ports = append(svc.Spec.Ports, corev1.ServicePort{
						Name:     communityIdName,
						Port:     int32(port),
						Protocol: corev1.Protocol(protocol),
					})
					updateService = true
				}
				if ok := endpointPortProtocol[portProtoStruct]; !ok {
					ep.Subsets[0].Ports = append(ep.Subsets[0].Ports, corev1.EndpointPort{
						Name:     communityIdName,
						Port:     int32(port),
						Protocol: corev1.Protocol(protocol),
					})
					updateEndpoint = true
				}

				if updateService {
					_, err := k8sClient.KubernetesClientSet.CoreV1().Services(svc.Namespace).Update(context.Background(), &svc, metav1.UpdateOptions{})
					if err != nil {
						return err
					}
				}
				if updateEndpoint {
					_, err := k8sClient.KubernetesClientSet.CoreV1().Endpoints(svc.Namespace).Update(context.Background(), ep, metav1.UpdateOptions{})
					if err != nil {
						return err
					}
				}
			}
		}

	}

	return nil
}

func subscriptionMessage(name, namespace string) string {

	return fmt.Sprintf(`<iq type="set" from="worker-3" to="network-control@contrailsystems.com/bgp-peer" id="subscribe3">
		<pubsub xmlns="http://jabber.org/protocol/pubsub">
		<subscribe node="default-domain:%s:%s:%s">
		</subscribe>
		</pubsub>
		</iq>`, namespace, name, name)
}

func reconcileRP(rp *ccapi.RoutingPolicy, serviceCommunity api.ServiceCommunity, k8sClient *k8s.Client) error {
	vn, err := k8sClient.ContrailClientSet.CoreV1().VirtualNetworks(serviceCommunity.VirtualNetworkNamespace).Get(context.Background(), serviceCommunity.VirtualNetworkName, metav1.GetOptions{})
	if err != nil {
		return nil
	}
	existingRP, err := k8sClient.ContrailClientSet.CoreV1().RoutingPolicies(rp.Namespace).Get(context.Background(), rp.Name, metav1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
	}
	if existingRP == nil {
		_, err := k8sClient.ContrailClientSet.CoreV1().RoutingPolicies(rp.Namespace).Create(context.Background(), rp, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	} else {
		_, err := k8sClient.ContrailClientSet.CoreV1().RoutingPolicies(rp.Namespace).Update(context.Background(), rp, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}
	refExists := false
	for _, rpRef := range vn.Spec.RoutingPolicyReferences {
		if rpRef.Name == rp.Name && rpRef.Namespace == rp.Namespace {
			refExists = true
		}
	}
	if !refExists {
		vn.Spec.RoutingPolicyReferences = append(vn.Spec.RoutingPolicyReferences, ccapi.ResourceReference{
			ObjectReference: corev1.ObjectReference{
				Name:       rp.Name,
				Namespace:  rp.Namespace,
				Kind:       "RoutingPolicy",
				APIVersion: "core.contrail.juniper.net/v1",
			},
		})
		_, err := k8sClient.ContrailClientSet.CoreV1().VirtualNetworks(vn.Namespace).Update(context.Background(), vn, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}

	return nil
}

func serviceToRP(serviceCommunity api.ServiceCommunity, k8sClient *k8s.Client) (*ccapi.RoutingPolicy, error) {
	service, err := k8sClient.KubernetesClientSet.CoreV1().Services(serviceCommunity.ServiceNamespace).Get(context.Background(), serviceCommunity.ServiceName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	var addrPortProtoMap = make(map[string][]struct {
		port     int32
		protocol string
	})
	var prefixList = []ccapi.PrefixMatchType{}
	var communityList []string
	if len(service.Spec.ExternalIPs) == 0 {
		return nil, fmt.Errorf("no externalIP configured on service")
	}
	for _, externalIP := range service.Spec.ExternalIPs {
		for _, port := range service.Spec.Ports {
			addrPortProtoMap[externalIP] = append(addrPortProtoMap[externalIP], struct {
				port     int32
				protocol string
			}{port.Port, string(port.Protocol)})
		}
	}
	for prefix, portProtoList := range addrPortProtoMap {
		prefixMatchType := ccapi.PrefixMatchType{
			Prefix: fmt.Sprintf("%s/32", prefix),
		}
		prefixList = append(prefixList, prefixMatchType)
		for _, portProto := range portProtoList {
			community := encodePortProtocolCommunity(portProto.port, portProto.protocol, serviceCommunity.Community)
			communityList = append(communityList, community)
		}
	}

	return &ccapi.RoutingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      service.Name,
			Namespace: serviceCommunity.VirtualNetworkNamespace,
		},
		Spec: ccapi.RoutingPolicySpec{
			RoutingPolicyEntries: ccapi.PolicyStatementType{
				Term: []ccapi.PolicyTermType{{
					TermMatchCondition: ccapi.TermMatchConditionType{
						Prefix: prefixList,
					},
					TermActionList: ccapi.TermActionListType{
						Update: ccapi.ActionUpdateType{
							Community: ccapi.ActionCommunityType{
								Add: ccapi.CommunityListType{
									Community: communityList,
								},
							},
						},
					},
				}},
			},
		},
	}, nil
}

func encodePortProtocolCommunity(port int32, protocol, community string) string {
	communityInt, _ := strconv.Atoi(community)
	communityInt16 := uint16(communityInt)
	port16 := int16(port)
	switch protocol {
	case "TCP":
		communityInt16 = setBit(communityInt16, 15)
	}
	return fmt.Sprintf("%d:%d", communityInt16, port16)
}

func decodePortProtocolCommunity(encodedCommunity string) (int32, uint16, string) {
	var proto string
	commList := strings.Split(encodedCommunity, ":")
	communityProtocolInt, _ := strconv.Atoi(commList[0])
	communityProtocol := uint16(communityProtocolInt)
	protoBit := communityProtocol & 32768
	if protoBit == 0 {
		proto = "UDP"
	} else {
		proto = "TCP"
	}
	communityId := clearBit(communityProtocol, 15)
	port, _ := strconv.Atoi(commList[1])
	return int32(port), communityId, proto
}

func setBit(n uint16, pos uint) uint16 {
	n |= (1 << pos)
	return n
}

func clearBit(n uint16, pos uint) uint16 {
	mask := ^(1 << pos)
	n &= uint16(mask)
	return n
}
