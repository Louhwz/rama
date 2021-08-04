package utils

import (
	"time"

	networkingv1 "github.com/oecp/rama/pkg/apis/networking/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ReasonClusterReady        = "ClusterReady"
	ReasonClusterNotReachable = "ClusterNotReachable"
	ReasonClusterNotReady     = "ClusterNotReady"
	ReasonClusterReachable    = "ClusterReachable"

	MsgHealthzNotOk = "/healthz responded without ok"
	MsgHealthzOk    = "/healthz responded with ok"
)

func NewClusterReady() networkingv1.ClusterCondition {
	return networkingv1.ClusterCondition{
		Type:               networkingv1.ClusterReady,
		Status:             corev1.ConditionTrue,
		LastProbeTime:      metav1.NewTime(time.Now()),
		LastTransitionTime: nil,
		Reason:             StringPtr(ReasonClusterReady),
		Message:            StringPtr(MsgHealthzOk),
	}
}

func NewClusterOffline(err error) networkingv1.ClusterCondition {
	return networkingv1.ClusterCondition{
		Type:               networkingv1.ClusterOffline,
		Status:             corev1.ConditionTrue,
		LastProbeTime:      metav1.NewTime(time.Now()),
		LastTransitionTime: nil,
		Reason:             StringPtr(ReasonClusterNotReachable),
		Message:            StringPtr(err.Error()),
	}
}

func NewClusterNotReady(err error) networkingv1.ClusterCondition {
	return networkingv1.ClusterCondition{
		Type:               networkingv1.ClusterReady,
		Status:             corev1.ConditionFalse,
		LastProbeTime:      metav1.NewTime(time.Now()),
		LastTransitionTime: nil,
		Reason:             StringPtr(ReasonClusterNotReady),
		Message:            StringPtr(err.Error()),
	}
}

func NewClusterNotOffline() networkingv1.ClusterCondition {
	return networkingv1.ClusterCondition{
		Type:               networkingv1.ClusterOffline,
		Status:             corev1.ConditionFalse,
		LastProbeTime:      metav1.NewTime(time.Now()),
		LastTransitionTime: nil,
		Reason:             StringPtr(ReasonClusterReachable),
		Message:            StringPtr(MsgHealthzNotOk),
	}
}

func StringPtr(s string) *string {
	return &s
}
