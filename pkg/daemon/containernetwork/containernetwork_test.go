package containernetwork

import (
	"github.com/stretchr/testify/assert"
	"github.com/vishvananda/netlink"
	"net"
	"testing"
)

func TestParseMac(t *testing.T) {
	macAddress, err := net.ParseMAC(ContainerHostLinkMac)
	t.Log(macAddress, err)
}

func TestGetDefaultRoute(t *testing.T) {
	defaultGatewayIf, err := GetDefaultInterface(netlink.FAMILY_V4)
	assert.Nil(t, err)
	t.Log(defaultGatewayIf.Name)
}
