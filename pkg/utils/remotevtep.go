package utils

import "fmt"

const remoteVtepNameFormat = "cluster%v.%v"

func GenRemoteVtepName(clusterID uint32, nodeName string) string {
	return fmt.Sprintf(remoteVtepNameFormat, clusterID, nodeName)
}
