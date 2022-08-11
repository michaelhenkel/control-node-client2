package control

import (
	"bufio"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"net"
	"os"
	"strings"

	"github.com/michaelhenkel/control-node-client2/pkg/schema"

	api "github.com/michaelhenkel/control-node-client2/pkg/schema"
)

type Client struct {
	conn         net.Conn
	messageQueue messageQueue
}

func (c *Client) Watch(callbackChan chan api.PrefixCommunity) error {
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
	msgObj := &schema.Message{}
	if err := xml.Unmarshal([]byte(msg), msgObj); err != nil {
		fmt.Println("err", err)
	} else {
		for _, item := range msgObj.Event.Items.Item {
			if item.Entry.CommunityTagList.CommunityTag != "" {
				fmt.Printf("%s:%s:%s\n", item.Entry.VirtualNetwork, item.Entry.Nlri.Address, item.Entry.CommunityTagList.CommunityTag)
			}
		}
	}

}

func (c *Client) handle(message []byte) {
	fmt.Println(string(message))
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

func NewClient(addr string) *Client {
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
	}
}

type messageQueue struct {
	messages map[int]string
}
