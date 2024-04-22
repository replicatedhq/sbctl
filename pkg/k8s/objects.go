package k8s

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func GetEmptyEventList() *corev1.EventList {
	r := &corev1.EventList{
		Items: []corev1.Event{},
	}
	r.GetObjectKind().SetGroupVersionKind(schema.GroupVersionKind{
		Version: "v1",
		Kind:    "EventList",
	})
	return r
}

func GetEmptyPodList() *corev1.PodList {
	r := &corev1.PodList{
		Items: []corev1.Pod{},
	}
	r.GetObjectKind().SetGroupVersionKind(schema.GroupVersionKind{
		Version: "v1",
		Kind:    "PodList",
	})
	return r
}

func GetEmptyLimitRangeList() *corev1.LimitRangeList {
	r := &corev1.LimitRangeList{
		Items: []corev1.LimitRange{},
	}
	r.GetObjectKind().SetGroupVersionKind(schema.GroupVersionKind{
		Version: "v1",
		Kind:    "LimitRangeList",
	})
	return r
}

func GetEmptyServiceList() *corev1.ServiceList {
	r := &corev1.ServiceList{
		Items: []corev1.Service{},
	}
	r.GetObjectKind().SetGroupVersionKind(schema.GroupVersionKind{
		Version: "v1",
		Kind:    "ServiceList",
	})
	return r
}

func GetEmptyPersistentVolumeClaimList() *corev1.PersistentVolumeClaimList {
	r := &corev1.PersistentVolumeClaimList{
		Items: []corev1.PersistentVolumeClaim{},
	}
	r.GetObjectKind().SetGroupVersionKind(schema.GroupVersionKind{
		Version: "v1",
		Kind:    "PersistentVolumeClaimList",
	})
	return r
}

func GetEmptyConfigMapList() *corev1.ConfigMapList {
	r := &corev1.ConfigMapList{
		Items: []corev1.ConfigMap{},
	}
	r.GetObjectKind().SetGroupVersionKind(schema.GroupVersionKind{
		Version: "v1",
		Kind:    "ConfigMapList",
	})
	return r
}
