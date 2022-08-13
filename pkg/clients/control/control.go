package control

import (
	"bufio"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"net"
	"os"
	"regexp"
	"strings"

	"github.com/michaelhenkel/control-node-client2/pkg/schema"

	api "github.com/michaelhenkel/control-node-client2/pkg/schema"
)

type Client struct {
	conn         net.Conn
	messageQueue messageQueue
	callbackChan chan api.PrefixCommunity
}

func (c *Client) Watch() error {
	msg := make([]byte, 40960)
	for {
		_, err := c.conn.Read(msg)
		if err == io.EOF {
			fmt.Println("closing connection")
			return nil
		}

		if err != nil {
			return err
		}
		msg = bytes.Trim(msg, "\x00")
		c.handle(msg)
		msg = make([]byte, 40960)
	}
}

func (c *Client) Write(msg string) error {
	_, err := c.conn.Write([]byte(msg))
	return err
}

func (c *Client) newMessage(line string) {
	messagesInQueue := len(c.messageQueue.messages)
	c.messageQueue.messages[messagesInQueue] = line
}

func (c *Client) appendMessage(line string) {
	messagesInQueue := len(c.messageQueue.messages) - 1
	c.messageQueue.messages[messagesInQueue] = fmt.Sprintf("%s\n%s", c.messageQueue.messages[messagesInQueue], line)
}

func (c *Client) endMessage(line string) string {
	messagesInQueue := len(c.messageQueue.messages) - 1
	c.messageQueue.messages[messagesInQueue] = fmt.Sprintf("%s\n%s", c.messageQueue.messages[messagesInQueue], line)
	msg := c.messageQueue.messages[messagesInQueue]
	delete(c.messageQueue.messages, messagesInQueue)
	return msg
}

func (c *Client) processMessage(msg string) {
	fmt.Println(msg)
	msgObj := &schema.Message{}
	if err := xml.Unmarshal([]byte(msg), msgObj); err != nil {
		fmt.Println("err", err)
	} else {
		r, err := regexp.Compile(`\d/\d/(.*?):(.*?):(.*?):(.*?)`)
		if err != nil {
			fmt.Println(err)
		}
		var networkNamespace, networkName string
		res := r.FindStringSubmatch(msgObj.Event.Items.Node)
		if len(res) > 3 {
			networkName = res[3]
			networkNamespace = res[2]
		}
		if msgObj.Event.Items.Retract.ID != "" {
			prefixList := strings.Split(msgObj.Event.Items.Retract.ID, "/")
			if net.ParseIP(prefixList[0]).To4() != nil {
				c.callbackChan <- api.PrefixCommunity{
					Prefix:                        msgObj.Event.Items.Retract.ID,
					Community:                     "0",
					OriginVirtualNetworkNamespace: networkNamespace,
					OriginVirtualNetworkName:      networkName,
					Action:                        api.Del,
				}
			}
		} else {
			for _, item := range msgObj.Event.Items.Item {
				prefixList := strings.Split(item.Entry.Nlri.Address, "/")
				if net.ParseIP(prefixList[0]).To4() != nil {
					if item.Entry.CommunityTagList.CommunityTag != "" {
						fmt.Printf("%s:%s:%s\n", item.Entry.VirtualNetwork, item.Entry.Nlri.Address, item.Entry.CommunityTagList.CommunityTag)
						c.callbackChan <- api.PrefixCommunity{
							Prefix:                        item.Entry.Nlri.Address,
							Community:                     item.Entry.CommunityTagList.CommunityTag,
							OriginVirtualNetworkNamespace: networkNamespace,
							OriginVirtualNetworkName:      networkName,
							Action:                        api.Add,
						}
					}
				}
			}
		}
	}

}

func (c *Client) handle(message []byte) {
	//fmt.Println(string(message))
	if message[0] == 200 && message[1] == 128 {
		ka := []byte{message[0], message[1]}
		c.Write(string(ka))
	} else {
		isStream := strings.HasPrefix(string(message), "<stream:stream")
		if !isStream {
			scanner := bufio.NewScanner(bytes.NewReader(message))
			for scanner.Scan() {
				line := scanner.Text()
				if line != "" {
					start := strings.HasPrefix(line, "<message")
					end := strings.HasPrefix(line, "</message")
					if start {
						c.newMessage(line)
					} else if end {
						finalMsg := c.endMessage(line)
						c.processMessage(finalMsg)
					} else {
						c.appendMessage(line)
					}
				}

			}
		}
	}
}

func NewClient(addr string, callBackChan chan api.PrefixCommunity) *Client {
	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		println("ResolveTCPAddr failed:", err.Error())
		os.Exit(1)
	}

	conn, err := net.DialTCP("tcp", nil, tcpAddr)
	if err != nil {
		println("Dial failed:", err.Error())
		os.Exit(1)
	}
	return &Client{
		conn: conn,
		messageQueue: messageQueue{
			messages: map[int]string{},
		},
		callbackChan: callBackChan,
	}
}

type messageQueue struct {
	messages map[int]string
}
