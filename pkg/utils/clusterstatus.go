package utils

import (
	networkingv1 "github.com/oecp/rama/pkg/apis/networking/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	TypeHealthCheck      networkingv1.ClusterConditionType = "HealthCheck"
	TypeTwoWayConn       networkingv1.ClusterConditionType = "TwoWayConnection"
	TypeSameOverlayNetID networkingv1.ClusterConditionType = "SameOverlayNetID"

	ReasonClusterReady        = "ClusterReady"
	ReasonClusterNotReachable = "ClusterNotReachable"
	ReasonClusterNotReady     = "ClusterNotReady"
	ReasonClusterReachable    = "ClusterReachable"
	ReasonDoubleConn          = "BothSetRemoteCluster"
	ReasonNotDoubleConn       = "RemoteNotSetRemoteCluster"
	ReasonSameOverlayNetID    = "SameOverlayNetIDReady"
	ReasonNotSameOverlayNetID = "SameOverlayNetIDNotReady"

	MsgHealthzNotOk     = "/healthz responded without ok"
	MsgHealthzOk        = "/healthz responded with ok"
	MsgDoubleConnOk     = "Both Clusters have created remote cluster"
	MsgDoubleConnNotOk  = "Remote Clusters have not apply remote-cluster-cr about local cluster"
	MsgSameOverlayNetID = "Both clusters have same overlay net id"
)

func NewHealthCheckReady() networkingv1.ClusterCondition {
	cur := metav1.Now()
	return networkingv1.ClusterCondition{
		Type:               TypeHealthCheck,
		Status:             corev1.ConditionTrue,
		LastProbeTime:      cur,
		LastTransitionTime: &cur,
		Reason:             StringPtr(ReasonClusterReady),
		Message:            StringPtr(MsgHealthzOk),
	}
}

func NewDoubleConnReady() networkingv1.ClusterCondition {
	cur := metav1.Now()
	return networkingv1.ClusterCondition{
		Type:               TypeTwoWayConn,
		Status:             corev1.ConditionTrue,
		LastProbeTime:      cur,
		LastTransitionTime: &cur,
		Reason:             StringPtr(ReasonDoubleConn),
		Message:            StringPtr(MsgDoubleConnOk),
	}
}

func NewDoubleConnNotReady() networkingv1.ClusterCondition {
	cur := metav1.Now()
	return networkingv1.ClusterCondition{
		Type:               TypeTwoWayConn,
		Status:             corev1.ConditionFalse,
		LastProbeTime:      cur,
		LastTransitionTime: &cur,
		Reason:             StringPtr(ReasonNotDoubleConn),
		Message:            StringPtr(MsgDoubleConnNotOk),
	}
}

func NewOverlayNetIDReady() networkingv1.ClusterCondition {
	cur := metav1.Now()
	return networkingv1.ClusterCondition{
		Type:               TypeSameOverlayNetID,
		Status:             corev1.ConditionTrue,
		LastProbeTime:      cur,
		LastTransitionTime: &cur,
		Reason:             StringPtr(ReasonSameOverlayNetID),
		Message:            StringPtr(MsgSameOverlayNetID),
	}
}

func NewOverlayNetIDNotReady(err error) networkingv1.ClusterCondition {
	cur := metav1.Now()
	return networkingv1.ClusterCondition{
		Type:               TypeSameOverlayNetID,
		Status:             corev1.ConditionFalse,
		LastProbeTime:      cur,
		LastTransitionTime: &cur,
		Reason:             StringPtr(ReasonNotSameOverlayNetID),
		Message:            StringPtr(err.Error()),
	}
}

func NewClusterOffline(err error) networkingv1.ClusterCondition {
	cur := metav1.Now()
	return networkingv1.ClusterCondition{
		Type:               networkingv1.ClusterOffline,
		Status:             corev1.ConditionTrue,
		LastProbeTime:      cur,
		LastTransitionTime: &cur,
		Reason:             StringPtr(ReasonClusterNotReachable),
		Message:            StringPtr(err.Error()),
	}
}

func NewHealthCheckNotReady(err error) networkingv1.ClusterCondition {
	cur := metav1.Now()
	return networkingv1.ClusterCondition{
		Type:               TypeHealthCheck,
		Status:             corev1.ConditionFalse,
		LastProbeTime:      cur,
		LastTransitionTime: &cur,
		Reason:             StringPtr(ReasonClusterNotReady),
		Message:            StringPtr(err.Error()),
	}
}

func NewClusterNotOffline() networkingv1.ClusterCondition {
	cur := metav1.Now()
	return networkingv1.ClusterCondition{
		Type:               networkingv1.ClusterOffline,
		Status:             corev1.ConditionFalse,
		LastProbeTime:      cur,
		LastTransitionTime: &cur,
		Reason:             StringPtr(ReasonClusterReachable),
		Message:            StringPtr(MsgHealthzNotOk),
	}
}

func StringPtr(s string) *string {
	return &s
}
