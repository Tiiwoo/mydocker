package network

import (
	"fmt"
	"net"
	"os/exec"
	"strings"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
)

type BridgeNetworkDriver struct{}

func (d *BridgeNetworkDriver) Name() string {
	return "bridge"
}

func (d *BridgeNetworkDriver) Create(subnet, name string) (*Network, error) {
	ip, ipRange, _ := net.ParseCIDR(subnet)
	ipRange.IP = ip
	n := &Network{
		Name:    name,
		IPRange: ipRange,
		Driver:  d.Name(),
	}
	// 配置 Linux Bridge
	err := d.initBridge(n)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to create bridge network")
	}
	return n, nil
}

func (d *BridgeNetworkDriver) Delete(network Network) error {
	// 根据名字找到对应的 Bridge 设备
	br, err := netlink.LinkByName(network.Name)
	if err != nil {
		return err
	}
	// 删除网络对应的 Linux Bridge 设备
	return netlink.LinkDel(br)
}

func (d *BridgeNetworkDriver) Connect(network *Network, endpoint *Endpoint) error {
	bridgeName := network.Name
	// 通过接口名获取到 Linux Bridge 接口的对象和接口属性
	br, err := netlink.LinkByName(bridgeName)
	if err != nil {
		return err
	}
	// 创建 Veth 接口的配置
	la := netlink.NewLinkAttrs()
	// 由于 Linux 接口名的限制,取 endpointID 的前 5 个
	la.Name = endpoint.ID[:5]
	// 通过设置 Veth 接口 master 属性，设置这个 Veth 的一端挂载到网络对应的 Linux Bridge
	la.MasterIndex = br.Attrs().Index
	// 创建 Veth 对象，通过 PeerName 配置 Veth 另外一端的接口名
	// 配置 Veth 另外 端的名字 cif {endpoint ID 的前 5 位｝
	endpoint.Device = netlink.Veth{
		LinkAttrs: la,
		PeerName:  "cif-" + endpoint.ID[:5],
	}
	// 通过 LinkAdd 方法创建 Veth 接口
	// Veth 令一端已经挂载 Linux Bridge 上
	if err = netlink.LinkAdd(&endpoint.Device); err != nil {
		return fmt.Errorf("error Add Endpoint Device: %v", err)
	}
	// 启动 Veth
	// ip link set xxx up
	if err = netlink.LinkSetUp(&endpoint.Device); err != nil {
		return fmt.Errorf("error Add Endpoint Device: %v", err)
	}
	return nil
}

func (d *BridgeNetworkDriver) Disconnect(network Network, endpoint *Endpoint) error {
	return nil
}

// 初始化 Linux Bridge
/*
Linux Bridge 初始化流程如下：
1. 创建 Bridge 虚拟设备
2. 设置 Bridge 设备地址和路由
3. 启动 Bridge 设备
4. 设置 iptables SNAT 规则
*/
func (d *BridgeNetworkDriver) initBridge(n *Network) error {
	bridgeName := n.Name
	// 1. 创建 Bridge 虚拟设备
	if err := createBridgeInterface(bridgeName); err != nil {
		return errors.Wrapf(err, "Failed to create bridge %s", bridgeName)
	}
	// 2. 设置 Bridge 设备地址和路由
	gatewayIP := *n.IPRange
	gatewayIP.IP = n.IPRange.IP
	if err := setInterfaceIP(bridgeName, gatewayIP.String()); err != nil {
		return errors.Wrapf(err, "Error set bridge ip: %s on bridge: %s", gatewayIP.String(), bridgeName)
	}
	// 3. 启动 Bridge 设备
	if err := setInterfaceUp(bridgeName); err != nil {
		return errors.Wrapf(err, "Failed to set %s up", bridgeName)
	}
	// 4. 设置 iptables SNAT 规则
	if err := setupIptables(bridgeName, n.IPRange); err != nil {
		return errors.Wrapf(err, "Failed to set up iptables for %s", bridgeName)
	}
	return nil
}

// 创建 Bridge 设备
// ip link add xxxx
func createBridgeInterface(bridgeName string) error {
	// 检查是否有同名的存在
	_, err := net.InterfaceByName(bridgeName)
	if err == nil || !strings.Contains(err.Error(), "no such network interface") {
		return err
	}

	// 创建 *netlink.Bridge 对象
	la := netlink.NewLinkAttrs()
	la.Name = bridgeName
	// 使用 la 这个属性来创建 Bridge
	br := &netlink.Bridge{LinkAttrs: la}
	// 使用 net link LinkAdd 方法，创建 Bridge 虚拟设备
	// 等同于 ip link add xxx
	if err = netlink.LinkAdd(br); err != nil {
		return errors.Wrapf(err, "create bridge %s error", bridgeName)
	}
	return nil
}

// 设置 IP
// ip addr add xxx
func setInterfaceIP(name, rawIP string) error {
	var iface netlink.Link
	var err error
	for i := 0; i < 2; i++ {
		iface, err = netlink.LinkByName(name)
		if err == nil {
			break
		}
		log.Debugf("error retrieving new bridge netlink link [ %s ]... retrying", name)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		return errors.Wrap(err, "abandoning retrieving the new bridge link from netlink, Run [ ip link ] to troubleshoot")
	}
	// 由于 netlink.ParseIPNet 是对 net.ParseCIDR 的一个封装，因此可以将 net.PareCIDR 中返回的 IP 进行整合
	// 返回值中的 ipNet 既包含了网段的信息，192 168.0.0/24 ，也包含了原始的IP 192.168.0.1
	ipNet, err := netlink.ParseIPNet(rawIP)
	if err != nil {
		return err
	}
	// 通过 netlink.AddrAdd 给网络接口配置地址，相当于 ip addr add xxx 命令
	// 同时如果配置了地址所在网段的信息，例如 192.168.0.0/24
	// 还会配置路由表 192.168.0.0/24 转发到这 testbridge 的网络接口上
	addr := &netlink.Addr{IPNet: ipNet}
	return netlink.AddrAdd(iface, addr)
}

func setInterfaceUp(interfaceName string) error {
	link, err := netlink.LinkByName(interfaceName)
	if err != nil {
		return errors.Wrapf(err, "error retrieving a link named [ %s ]:", link.Attrs().Name)
	}
	// ip link set xxx up
	if err = netlink.LinkSetUp(link); err != nil {
		return errors.Wrapf(err, "nabling interface for %s", interfaceName)
	}
	return nil
}

// 设置 iptables 对应 bridge MASQUERADE 规则
// iptables -t nat -A POSTROUTING -s {subnet} -o {deviceName} -j MASQUERADE
// iptables -t nat -A POSTROUTING -s 172.18.0.0/24 -o eth0 -j MASQUERADE
func setupIptables(bridgeName string, subnet *net.IPNet) error {
	iptablesCmd := fmt.Sprintf("-t nat -A POSTROUTING -s %s ! -o %s -j MASQUERADE", subnet.String(), bridgeName)
	cmd := exec.Command("iptables", strings.Split(iptablesCmd, " ")...)
	output, err := cmd.Output()
	if err != nil {
		log.Errorf("iptables Output, %v", output)
	}
	return err
}
