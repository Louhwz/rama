/*
  Copyright 2021 The Rama Authors.

  Licensed under the Apache License, Version 2.0 (the "License");
  you may not use this file except in compliance with the License.
  You may obtain a copy of the License at

      http://www.apache.org/licenses/LICENSE-2.0

  Unless required by applicable law or agreed to in writing, software
  distributed under the License is distributed on an "AS IS" BASIS,
  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
  See the License for the specific language governing permissions and
  limitations under the License.
*/

package transform

import (
	"net"

	v1 "github.com/oecp/rama/pkg/apis/networking/v1"
	ipamtypes "github.com/oecp/rama/pkg/ipam/types"
	"github.com/oecp/rama/pkg/utils"
)

func TransferSubnetForIPAM(in *v1.Subnet) *ipamtypes.Subnet {
	_, cidr, _ := net.ParseCIDR(in.Spec.Range.CIDR)

	return ipamtypes.NewSubnet(in.Name,
		in.Spec.Network,
		in.Spec.NetID,
		net.ParseIP(in.Spec.Range.Start),
		net.ParseIP(in.Spec.Range.End),
		net.ParseIP(in.Spec.Range.Gateway),
		cidr,
		utils.StringSliceToMap(in.Spec.Range.ReservedIPs),
		utils.StringSliceToMap(in.Spec.Range.ExcludeIPs),
		net.ParseIP(in.Status.LastAllocatedIP),
		v1.IsPrivateSubnet(&in.Spec),
		v1.IsIPv6Subnet(&in.Spec),
	)
}

func TransferNetworkForIPAM(in *v1.Network) *ipamtypes.Network {
	return ipamtypes.NewNetwork(in.Name,
		in.Spec.NetID,
		in.Status.LastAllocatedSubnet,
		ipamtypes.ParseNetworkTypeFromString(string(v1.GetNetworkType(in))),
	)
}

func TransferIPInstanceForIPAM(in *v1.IPInstance) *ipamtypes.IP {
	return &ipamtypes.IP{
		Address:      utils.StringToIPNet(in.Spec.Address.IP),
		Gateway:      net.ParseIP(in.Spec.Address.Gateway),
		NetID:        in.Spec.Address.NetID,
		Subnet:       in.Spec.Subnet,
		Network:      in.Spec.Network,
		PodName:      in.Status.PodName,
		PodNamespace: in.Status.PodNamespace,
		Status:       string(in.Status.Phase),
	}
}
