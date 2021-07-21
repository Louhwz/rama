package vxlan

import (
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vishvananda/netlink"
)

func TestNeighList(t *testing.T) {
	link, err := netlink.LinkByName("eth0.vxlan4")
	assert.Nil(t, err)
	fdbEntryList, err := netlink.NeighList(link.Attrs().Index, syscall.AF_BRIDGE)
	for _, entry := range fdbEntryList {
		t.Log(entry, entry.IP)

	}
}
