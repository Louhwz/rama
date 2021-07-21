package controller

import (
	"net"
	"testing"

	"github.com/oecp/rama/pkg/daemon/containernetwork"
	"github.com/stretchr/testify/assert"
	"github.com/vishvananda/netlink"
)

func TestListLocalAddressExceptLink(t *testing.T) {
	var vxlanLinkName = "eth0.vxlan4"
	existAllAddrList, err := containernetwork.ListLocalAddressExceptLink(vxlanLinkName)
	assert.Nil(t, err)
	t.Log(existAllAddrList)
}

func TestLinkList(t *testing.T) {
	links, err := LinkList()
	assert.Nil(t, err)
	t.Log(links)
}
