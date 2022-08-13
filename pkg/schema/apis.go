package schema

type ServiceCommunity struct {
	VirtualNetworkName      string
	VirtualNetworkNamespace string
	Community               string
	ServiceName             string
	ServiceNamespace        string
	Action                  Action
}

type PrefixCommunity struct {
	Prefix                        string
	Community                     string
	OriginVirtualNetworkName      string
	OriginVirtualNetworkNamespace string
	Action                        Action
}

type Action string

const (
	Add    Action = "add"
	Update Action = "update"
	Del    Action = "del"
)
