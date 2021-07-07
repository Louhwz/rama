package neigh

import (
	"net"
	"testing"
)

func TestManager_AddPodInfo(t *testing.T) {
	manager := CreateNeighManager(1)
	manager.AddPodInfo(net.IP{1,2,3,4}, "test")
	//manager.interfaceToIPSliceMap["test"][""]
	t.Logf("hello world")
	m := IPMap{"test": net.IP{1,2,3,4}}
	//m["test"] = net.IP{1,2,3,4}
	t.Log(m["test"])

}
