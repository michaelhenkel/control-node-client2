package main

import (
	"fmt"

	"github.com/michaelhenkel/control-node-client2/pkg/clients/control"
	"github.com/michaelhenkel/control-node-client2/pkg/clients/k8s"

	api "github.com/michaelhenkel/control-node-client2/pkg/schema"
)

var m = `<message from="network-control@contrailsystems.com" to="worker-3/bgp-peer">
<event xmlns="http://jabber.org/protocol/pubsub">
		<items node="1/1/default-domain:default:ext-vn2:ext-vn2">
										<item id="2.0.0.2/32">
						<entry>
								<nlri>
										<af>1</af>
										<safi>1</safi>
										<address>2.0.0.2/32</address>
								</nlri>
								<next-hops>
										<next-hop>
												<af>1</af>
												<address>10.1.1.3</address>
												<mac></mac>
												<label>47</label>
												<vni>0</vni>
												<tunnel-encapsulation-list>
														<tunnel-encapsulation>gre</tunnel-encapsulation>
														<tunnel-encapsulation>udp</tunnel-encapsulation>
												</tunnel-encapsulation-list>
												<virtual-network>default-domain:default:ext-vn2</virtual-network>
												<tag-list>
														<tag>65543</tag>
														<tag>524295</tag>
												</tag-list>
										</next-hop>
								</next-hops>
								<version>1</version>
								<virtual-network>default-domain:default:ext-vn2</virtual-network>
								<mobility seqno="1" sticky="false" />
								<sequence-number>1</sequence-number>
								<security-group-list />
								<community-tag-list>
										<community-tag>3:9</community-tag>
								</community-tag-list>
								<local-preference>200</local-preference>
								<med>100</med>
								<load-balance>
										<load-balance-fields />
										<load-balance-decision>field-hash</load-balance-decision>
								</load-balance>
								<sub-protocol>interface</sub-protocol>
						</entry>
				</item>
										<item id="10.235.65.0/32">
						<entry>
								<nlri>
										<af>1</af>
										<safi>1</safi>
										<address>10.235.65.0/32</address>
								</nlri>
								<next-hops>
										<next-hop>
												<af>1</af>
												<address>10.1.1.3</address>
												<mac></mac>
												<label>21</label>
												<vni>0</vni>
												<tunnel-encapsulation-list>
														<tunnel-encapsulation>gre</tunnel-encapsulation>
														<tunnel-encapsulation>udp</tunnel-encapsulation>
												</tunnel-encapsulation-list>
												<virtual-network>default-domain:contrail:link-local</virtual-network>
												<tag-list>
														<tag>65542</tag>
														<tag>393222</tag>
														<tag>458758</tag>
														<tag>524294</tag>
												</tag-list>
										</next-hop>
								</next-hops>
								<version>1</version>
								<virtual-network>default-domain:contrail:link-local</virtual-network>
								<mobility seqno="1" sticky="false" />
								<sequence-number>1</sequence-number>
								<security-group-list />
								<community-tag-list />
								<local-preference>200</local-preference>
								<med>100</med>
								<load-balance>
										<load-balance-fields />
										<load-balance-decision>field-hash</load-balance-decision>
								</load-balance>
								<sub-protocol>interface</sub-protocol>
						</entry>
				</item>
		</items>
</event>
</message>`

var msg1 = `<stream:stream from="worker-3" to="network-control@contrailsystems.com" version="1.0" xml:lang="en" xmlns="" xmlns:stream="http://etherx.jabber.org/streams"  >..<iq type="set" from="worker-3" to="network-control@contrailsystems.com/config">
<pubsub xmlns="http://jabber.org/protocol/pubsub">
<subscribe node="virtual-router:default-global-system-config:worker-3" />
</pubsub>
</iq>`

var msg2 = `<stream:stream from="worker-3" to="network-control@contrailsystems.com" version="1.0" xml:lang="en" xmlns="" xmlns:stream="http://etherx.jabber.org/streams"  >`
var msg3 = `<iq type="set" from="worker-3" to="network-control@contrailsystems.com/config">
<pubsub xmlns="http://jabber.org/protocol/pubsub">
<subscribe node="virtual-router:default-global-system-config:worker-3" />
</pubsub>
</iq>`

var msg4 = `<iq type="set" from="worker-3" to="network-control@contrailsystems.com/bgp-peer" id="subscribe3">
<pubsub xmlns="http://jabber.org/protocol/pubsub">
<subscribe node="default-domain:default:ext-vn2:ext-vn2">
</subscribe>
</pubsub>
</iq>`

func main() {

	stopChan := make(chan bool)
	controlClient := control.NewClient("10.233.65.14:5269")
	var k8sCallBackChan = make(chan api.VirtualNetworkCommunity)
	var controlCallBackChan = make(chan api.PrefixCommunity)
	var subscriberMap = make(map[api.VirtualNetworkCommunity]bool)

	go func() {
		fmt.Println("starting watch on callbackchan")
		for virtualNetworkCommunity := range k8sCallBackChan {
			if _, ok := subscriberMap[virtualNetworkCommunity]; !ok {
				subscriberMap[virtualNetworkCommunity] = true
				msg := subscriptionMessage(virtualNetworkCommunity.Name, virtualNetworkCommunity.Namespace)
				controlClient.Write(msg)
			}
		}
	}()

	go k8s.Watch("/home/mhenkel/admin.conf", k8sCallBackChan)
	go controlClient.Watch(controlCallBackChan)

	controlClient.Write(msg2)

	//bla := <-callBackChan
	//fmt.Println(bla)

	/*
		for virtualNetworkCommunity := range callBackChan {
			if _, ok := subscriberMap[virtualNetworkCommunity]; !ok {
				subscriberMap[virtualNetworkCommunity] = true
				msg := subscriptionMessage(virtualNetworkCommunity.Name, virtualNetworkCommunity.Namespace)
				controlClient.Write(msg)
			}
		}


	*/
	//controlClient.Write(msg4)
	<-stopChan
}

func subscriptionMessage(name, namespace string) string {
	return fmt.Sprintf(`<iq type="set" from="worker-3" to="network-control@contrailsystems.com/bgp-peer" id="subscribe3">
	<pubsub xmlns="http://jabber.org/protocol/pubsub">
	<subscribe node="default-domain:%s:%s:%s">
	</subscribe>
	</pubsub>
	</iq>`, namespace, name, name)
}
