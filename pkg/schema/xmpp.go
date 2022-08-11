package schema

import "encoding/xml"

type Messages struct {
	Text    string    `xml:",chardata"`
	Message []Message `xml:"message"`
}

type Message struct {
	XMLName xml.Name `xml:"message"`
	Text    string   `xml:",chardata"`
	From    string   `xml:"from,attr"`
	To      string   `xml:"to,attr"`
	Event   Event    `xml:"event"`
}

type Event struct {
	Text  string `xml:",chardata"`
	Xmlns string `xml:"xmlns,attr"`
	Items Items  `xml:"items"`
}

type Items struct {
	Text string `xml:",chardata"`
	Node string `xml:"node,attr"`
	Item []Item `xml:"item"`
}

type Item struct {
	Text  string `xml:",chardata"`
	ID    string `xml:"id,attr"`
	Entry Entry  `xml:"entry"`
}

type Entry struct {
	Text              string           `xml:",chardata"`
	Nlri              Nlri             `xml:"nlri"`
	NextHops          NextHops         `xml:"next-hops"`
	Version           string           `xml:"version"`
	VirtualNetwork    string           `xml:"virtual-network"`
	Mobility          Mobility         `xml:"mobility"`
	SequenceNumber    string           `xml:"sequence-number"`
	SecurityGroupList string           `xml:"security-group-list"`
	CommunityTagList  CommunityTagList `xml:"community-tag-list"`
	LocalPreference   string           `xml:"local-preference"`
	Med               string           `xml:"med"`
	LoadBalance       LoadBalance      `xml:"load-balance"`
	SubProtocol       string           `xml:"sub-protocol"`
}

type Nlri struct {
	Text    string `xml:",chardata"`
	Af      string `xml:"af"`
	Safi    string `xml:"safi"`
	Address string `xml:"address"`
}

type Mobility struct {
	Text   string `xml:",chardata"`
	Seqno  string `xml:"seqno,attr"`
	Sticky string `xml:"sticky,attr"`
}

type CommunityTagList struct {
	Text         string `xml:",chardata"`
	CommunityTag string `xml:"community-tag"`
}

type LoadBalance struct {
	Text                string `xml:",chardata"`
	LoadBalanceFields   string `xml:"load-balance-fields"`
	LoadBalanceDecision string `xml:"load-balance-decision"`
}

type NextHops struct {
	Text    string  `xml:",chardata"`
	NextHop NextHop `xml:"next-hop"`
}

type NextHop struct {
	Text                    string                  `xml:",chardata"`
	Af                      string                  `xml:"af"`
	Address                 string                  `xml:"address"`
	Mac                     string                  `xml:"mac"`
	Label                   string                  `xml:"label"`
	Vni                     string                  `xml:"vni"`
	TunnelEncapsulationList TunnelEncapsulationList `xml:"tunnel-encapsulation-list"`
	VirtualNetwork          string                  `xml:"virtual-network"`
	TagList                 TagList                 `xml:"tag-list"`
}

type TunnelEncapsulationList struct {
	Text                string   `xml:",chardata"`
	TunnelEncapsulation []string `xml:"tunnel-encapsulation"`
}

type TagList struct {
	Text string   `xml:",chardata"`
	Tag  []string `xml:"tag"`
}
