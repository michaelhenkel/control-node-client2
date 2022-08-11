package schema

type VirtualNetworkCommunity struct {
	Name      string
	Namespace string
	Community string
}

type PrefixCommunity struct {
	Prefix    string
	Community string
}
