package utils

import (
	"fmt"
	"strings"
)

const remoteSubnetNameFormat = "cluster%v.%v"

type RemoteSubnetName string

func GenRemoteSubnetName(clusterName string, subnetName string) string {
	return fmt.Sprintf(remoteSubnetNameFormat, clusterName, subnetName)
}

func ExtractSubnetName(remoteSubnetName string) string {
	s := strings.Split(remoteSubnetName, ".")
	if len(s) != 2 {
		return ""
	}
	return s[1]
}
