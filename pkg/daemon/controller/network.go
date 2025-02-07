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

package controller

import ramav1 "github.com/oecp/rama/pkg/apis/networking/v1"

// reconcile subnet and node info on node if network info changed
func (c *Controller) enqueueAddOrDeleteNetwork(obj interface{}) {
	network := obj.(*ramav1.Network)
	if ramav1.GetNetworkType(network) == ramav1.NetworkTypeOverlay {
		c.nodeQueue.Add(ActionReconcileNode)
	}
	c.subnetQueue.Add(ActionReconcileSubnet)
}

func (c *Controller) enqueueUpdateNetwork(oldObj, newObj interface{}) {
	oldNetwork := oldObj.(*ramav1.Network)
	newNetwork := newObj.(*ramav1.Network)

	if len(oldNetwork.Status.SubnetList) != len(newNetwork.Status.SubnetList) ||
		len(oldNetwork.Status.NodeList) != len(newNetwork.Status.NodeList) {
		c.subnetQueue.Add(ActionReconcileSubnet)
		return
	}

	for index, subnet := range oldNetwork.Status.SubnetList {
		if subnet != newNetwork.Status.SubnetList[index] {
			c.subnetQueue.Add(ActionReconcileSubnet)
			return
		}
	}

	for index, node := range oldNetwork.Status.NodeList {
		if node != newNetwork.Status.NodeList[index] {
			c.subnetQueue.Add(ActionReconcileSubnet)
			return
		}
	}
}
