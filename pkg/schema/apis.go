package schema

type ServiceCommunity struct {
	VirtualNetworkName      string
	VirtualNetworkNamespace string
	Community               string
	ServiceName             string
	ServiceNamespace        string
}

type PrefixCommunity struct {
	Prefix               string
	Community            string
	OriginVirtualNetwork string
}
