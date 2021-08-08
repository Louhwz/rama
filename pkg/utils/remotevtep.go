package utils

import (
	"fmt"

	networkingv1 "github.com/oecp/rama/pkg/apis/networking/v1"
	"github.com/oecp/rama/pkg/constants"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const remoteVtepNameFormat = "%v.%v"

func GenRemoteVtepName(clusterName string, nodeName string) string {
	if nodeName == "" || clusterName == "" {
		return ""
	}
	return fmt.Sprintf(remoteVtepNameFormat, clusterName, nodeName)
}

func NewRemoteVtep(clusterName, vtepIP, macAddr, nodeName string, podIPList []string) *networkingv1.RemoteVtep {
	vtep := &networkingv1.RemoteVtep{
		ObjectMeta: metav1.ObjectMeta{
			Name: GenRemoteVtepName(clusterName, nodeName),
			Labels: map[string]string{
				constants.LabelCluster: clusterName,
			},
		},
		Spec: networkingv1.RemoteVtepSpec{
			ClusterName: clusterName,
			NodeName:    nodeName,
			VtepIP:      vtepIP,
			VtepMAC:     macAddr,
			IPList:      podIPList,
		},
		Status: networkingv1.RemoteVtepStatus{
			LastModifyTime: metav1.Now(),
		},
	}
	return vtep
}
