package numaware

import (
	"context"
	"testing"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic/fake"
	"k8s.io/kubernetes/pkg/scheduler/framework"
)

func TestGetPodCPURequest(t *testing.T) {
	tests := []struct {
		name     string
		pod      *v1.Pod
		expected int64
	}{
		{
			name: "no resources",
			pod: &v1.Pod{
				Spec: v1.PodSpec{
					Containers: []v1.Container{{}},
				},
			},
			expected: 0,
		},
		{
			name: "single container integer cpu",
			pod: &v1.Pod{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{Resources: v1.ResourceRequirements{
							Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("2")},
						}},
					},
				},
			},
			expected: 2,
		},
		{
			name: "single container fractional cpu ceiling",
			pod: &v1.Pod{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{Resources: v1.ResourceRequirements{
							Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("2500m")},
						}},
					},
				},
			},
			expected: 3,
		},
		{
			name: "multiple containers sum ceiling",
			pod: &v1.Pod{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{Resources: v1.ResourceRequirements{
							Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1500m")},
						}},
						{Resources: v1.ResourceRequirements{
							Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("1000m")},
						}},
					},
				},
			},
			expected: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getPodCPURequest(tt.pod)
			if got != tt.expected {
				t.Errorf("getPodCPURequest() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func createTestTopologyCR(nodeName string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "topology.numa-aware-scheduler.io/v1alpha1",
			"kind":       "NumaTopology",
			"metadata": map[string]interface{}{
				"name": nodeName,
			},
			"spec": map[string]interface{}{
				"numaNodes": []interface{}{
					map[string]interface{}{
						"id": int64(0),
						"cpus": []interface{}{
							int64(0), int64(1), int64(2), int64(3),
						},
						"memoryCapacityBytes": int64(1024 * 1024 * 1024),
					},
					map[string]interface{}{
						"id": int64(1),
						"cpus": []interface{}{
							int64(4), int64(5), int64(6), int64(7), int64(8), int64(9), int64(10), int64(11),
						},
						"memoryCapacityBytes": int64(1024 * 1024 * 1024),
					},
				},
			},
		},
	}
}

func TestFilter(t *testing.T) {
	scheme := runtime.NewScheme()
	nodeName := "node-1"
	topologyCR := createTestTopologyCR(nodeName)

	dynClient := fake.NewSimpleDynamicClient(scheme, topologyCR)

	pl := &NumaAware{
		dynClient: dynClient,
	}

	tests := []struct {
		name       string
		pod        *v1.Pod
		nodeInfo   *framework.NodeInfo
		wantStatus *framework.Status
	}{
		{
			name: "pod fits in numa node 0",
			pod: &v1.Pod{
				Spec: v1.PodSpec{
					Containers: []v1.Container{{Resources: v1.ResourceRequirements{Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("3")}}}},
				},
			},
			nodeInfo:   framework.NewNodeInfo(),
			wantStatus: framework.NewStatus(framework.Success, ""),
		},
		{
			name: "pod fits in numa node 1 but not 0",
			pod: &v1.Pod{
				Spec: v1.PodSpec{
					Containers: []v1.Container{{Resources: v1.ResourceRequirements{Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("6")}}}},
				},
			},
			nodeInfo:   framework.NewNodeInfo(),
			wantStatus: framework.NewStatus(framework.Success, ""),
		},
		{
			name: "pod does not fit in any single numa node",
			pod: &v1.Pod{
				Spec: v1.PodSpec{
					Containers: []v1.Container{{Resources: v1.ResourceRequirements{Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("10")}}}},
				},
			},
			nodeInfo:   framework.NewNodeInfo(),
			wantStatus: framework.NewStatus(framework.Unschedulable, "Not enough CPUs on any single NUMA node"),
		},
		{
			name: "no topology data available",
			pod: &v1.Pod{
				Spec: v1.PodSpec{
					Containers: []v1.Container{{Resources: v1.ResourceRequirements{Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("6")}}}},
				},
			},
			nodeInfo:   framework.NewNodeInfo(),
			wantStatus: framework.NewStatus(framework.Success, ""), // missing topology should pass
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "no topology data available" {
				tt.nodeInfo.SetNode(&v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-2"}})
			} else {
				tt.nodeInfo.SetNode(&v1.Node{ObjectMeta: metav1.ObjectMeta{Name: nodeName}})
			}

			status := pl.Filter(context.Background(), nil, tt.pod, tt.nodeInfo)
			if status.Code() != tt.wantStatus.Code() {
				t.Errorf("Filter() status code = %v, want %v", status.Code(), tt.wantStatus.Code())
			}
		})
	}
}

func TestScore(t *testing.T) {
	scheme := runtime.NewScheme()
	nodeName := "node-1"
	topologyCR := createTestTopologyCR(nodeName)

	dynClient := fake.NewSimpleDynamicClient(scheme, topologyCR)

	pl := &NumaAware{
		dynClient: dynClient,
	}

	podFitPerfect := &v1.Pod{
		Spec: v1.PodSpec{
			Containers: []v1.Container{{Resources: v1.ResourceRequirements{Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("4")}}}},
		},
	}
	podFitLoose := &v1.Pod{
		Spec: v1.PodSpec{
			Containers: []v1.Container{{Resources: v1.ResourceRequirements{Requests: v1.ResourceList{v1.ResourceCPU: resource.MustParse("2")}}}},
		},
	}

	scoreFitPerfect, status := pl.Score(context.Background(), nil, podFitPerfect, nodeName)
	if !status.IsSuccess() {
		t.Fatalf("Score failed: %v", status)
	}

	scoreFitLoose, status := pl.Score(context.Background(), nil, podFitLoose, nodeName)
	if !status.IsSuccess() {
		t.Fatalf("Score failed: %v", status)
	}

	if scoreFitPerfect <= scoreFitLoose {
		t.Errorf("Expected perfect fit to score higher than loose fit, got perfect: %d, loose: %d", scoreFitPerfect, scoreFitLoose)
	}
}

func TestSimpleMethods(t *testing.T) {
	pl := &NumaAware{}
	if pl.Name() != Name {
		t.Errorf("Expected name %s, got %s", Name, pl.Name())
	}
	if pl.ScoreExtensions() != nil {
		t.Errorf("Expected nil ScoreExtensions")
	}
}
