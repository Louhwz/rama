package utils

import (
	"fmt"
	"strings"
)

const remoteSubnetNameFormat = "cluster%v.%v"

type RemoteSubnetName string

func GenRemoteSubnetName(clusterID uint32, subnetName string) string {
	return fmt.Sprintf(remoteSubnetNameFormat, clusterID, subnetName)
}

func ExtractSubnetName(remoteSubnetName string) string {
	s := strings.Split(remoteSubnetName, ".")
	if len(s) != 2 {
		return ""
	}
	return s[1]
}
