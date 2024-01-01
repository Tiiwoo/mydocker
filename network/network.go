package network

import (
	"net"

	"github.com/vishvananda/netlink"
)

type Endpoint struct {
	ID          string           `json:"id"`
	Device      netlink.Veth     `json:"dev"`
	IPAddress   net.IP           `json:"ip"`
	MacAddress  net.HardwareAddr `json:"mac"`
	Network     *Network
	PortMapping []string
}

type Network struct {
	Name    string     // 网络名
	IPRange *net.IPNet // ip 地址段
	Driver  string     // 网络驱动名
}

type Driver interface {
	Name() string
	Create(subnet, name string) (*Network, error)
	Delete(network Network) error
	Connect(network *Network, endpoint *Endpoint) error
	Disconnect(network Network, endpoint *Endpoint) error
}
