package dhcpserver

import (
	"fmt"

	"github.com/infrastructure-io/topohub/pkg/tools"
	"github.com/vishvananda/netlink"
)

const (
	// note, the letter length of the interface must be less than 15
	vlanInterfaceFormat = "%s.%d"
	//macvlanInterfaceFormat = "%s.topohub"
)

// setupInterface configures the network interface for DHCP server
func (s *dhcpServer) setupInterface() error {
	var interfaceName string
	baseInterface := s.subnet.Spec.Interface.Interface

	// 获取基础接口
	parent, err := netlink.LinkByName(baseInterface)
	if err != nil {
		return fmt.Errorf("base interface %s goes wrong: %v", baseInterface, err)
	}

	// 检查并确保基础接口是 up 状态
	if parent.Attrs().OperState == netlink.OperDown {
		s.log.Infof("Base interface %s is down, bringing it up", baseInterface)
		if err := netlink.LinkSetUp(parent); err != nil {
			return fmt.Errorf("failed to bring up base interface %s: %v", baseInterface, err)
		}
	}

	// 验证接口配置是否和主机网络冲突
	if err := tools.ValidateHostInterfaceSubnet(parent, &s.subnet.Spec.Interface); err != nil {
		return fmt.Errorf("interface configuration conflicts with host network: %v", err)
	}

	// 根据配置创建接口
	if s.subnet.Spec.Interface.VlanID != nil && *s.subnet.Spec.Interface.VlanID > 0 {
		s.log.Infof("Creating VLAN interface: %s.topohub.%d on vlan %d", baseInterface, *s.subnet.Spec.Interface.VlanID, *s.subnet.Spec.Interface.VlanID)

		interfaceName = fmt.Sprintf(vlanInterfaceFormat, baseInterface, *s.subnet.Spec.Interface.VlanID)
		if err := s.createVlanInterface(parent, interfaceName, *s.subnet.Spec.Interface.VlanID); err != nil {
			return err
		}
	} else {
		interfaceName = baseInterface
	}
	// } else {
	// 	s.log.Infof("Creating MACVLAN interface: %s.topohub", baseInterface)

	// 	interfaceName = fmt.Sprintf(macvlanInterfaceFormat, baseInterface)
	// 	if err := s.createMacvlanInterface(parent, interfaceName); err != nil {
	// 		return err
	// 	}
	// }

	// 配置 IP 地址
	return s.configureIP(interfaceName, s.subnet.Spec.Interface.IPv4)
}

// createVlanInterface creates a VLAN interface
func (s *dhcpServer) createVlanInterface(parent netlink.Link, name string, vlanID int32) error {
	// Check if interface already exists
	if link, err := netlink.LinkByName(name); err == nil {
		s.log.Debugf("Interface %s already exists", name)
		return netlink.LinkSetUp(link)
	}

	// Ensure VLAN ID is within valid range (0-4094)
	if vlanID < 0 || vlanID > 4094 {
		return fmt.Errorf("invalid VLAN ID %d: must be between 0 and 4094", vlanID)
	}

	// Convert to int explicitly to match netlink.Vlan field type
	// This is safe because we've already validated the range
	vlanId := int(vlanID)

	vlan := &netlink.Vlan{
		LinkAttrs: netlink.LinkAttrs{
			Name:        name,
			ParentIndex: parent.Attrs().Index,
		},
		VlanId: vlanId,
	}
	s.log.Infof("Creating VLAN interface: %+v", vlan)

	if len(name) > 15 {
		return fmt.Errorf("interface name %s is too long, it is must be less than 15 letters", name)
	}

	if err := netlink.LinkAdd(vlan); err != nil {
		return fmt.Errorf("failed to create VLAN interface: %v", err)
	}

	if err := netlink.LinkSetUp(vlan); err != nil {
		return fmt.Errorf("failed to set VLAN interface up: %v", err)
	}

	return nil
}

// // createMacvlanInterface creates a macvlan interface
// func (s *dhcpServer) createMacvlanInterface(parent netlink.Link, name string) error {
// 	// 检查接口是否已存在
// 	if link, err := netlink.LinkByName(name); err == nil {
// 		s.log.Debugf("Interface %s already exists", name)
// 		return netlink.LinkSetUp(link)
// 	}

// 	macvlan := &netlink.Macvlan{
// 		LinkAttrs: netlink.LinkAttrs{
// 			Name:        name,
// 			ParentIndex: parent.Attrs().Index,
// 		},
// 		Mode: netlink.MACVLAN_MODE_BRIDGE,
// 	}

// 	if err := netlink.LinkAdd(macvlan); err != nil {
// 		return fmt.Errorf("failed to create macvlan interface: %v", err)
// 	}

// 	if err := netlink.LinkSetUp(macvlan); err != nil {
// 		return fmt.Errorf("failed to set macvlan interface up: %v", err)
// 	}

// 	return nil
// }

// configureIP configures IP address on the interface
func (s *dhcpServer) configureIP(name, ipStr string) error {
	s.log.Infof("Configuring IP address: %s on interface %s", ipStr, name)

	// 获取接口
	link, err := netlink.LinkByName(name)
	if err != nil {
		return fmt.Errorf("interface %s not found: %v", name, err)
	}

	// 解析 IP 地址
	addr, err := netlink.ParseAddr(ipStr)
	if err != nil {
		return fmt.Errorf("invalid IP address %s: %v", ipStr, err)
	}

	// 检查是否已经配置了该 IP
	addrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
	if err != nil {
		return fmt.Errorf("failed to list addresses: %v", err)
	}

	for _, existing := range addrs {
		if existing.Equal(*addr) {
			s.log.Debugf("IP %s already configured on %s", ipStr, name)
			return nil
		}
	}

	// 添加 IP 地址
	if err := netlink.AddrAdd(link, addr); err != nil {
		return fmt.Errorf("failed to add IP address: %v", err)
	}

	return nil
}

// TODO cleanupAllInterface removes all topohub interfaces on the base interface
func (s *dhcpServer) cleanupAllInterface() error {
	return nil
}

// // checkInterface checks if the network interface exists
// func (s *dhcpServer) checkInterface(name string) error {
// 	_, err := netlink.LinkByName(name)
// 	if err != nil {
// 		return fmt.Errorf("interface %s does not exist", name)
// 	}
// 	return nil
// }
