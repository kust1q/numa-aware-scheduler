package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type NumaTopology struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec NumaTopologySpec `json:"spec"`
}

type NumaTopologySpec struct {
	NumaNodes []NumaNode `json:"numaNodes"`
}

type NumaNode struct {
	ID     int   `json:"id"`
	CPUs   []int `json:"cpus"`
	Memory int64 `json:"memoryCapacityBytes"`
}

type NumaTopologyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []NumaTopology `json:"items"`
}
