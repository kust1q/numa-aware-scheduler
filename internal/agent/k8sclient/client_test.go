package k8sclient

import (
	"context"
	"testing"

	v1 "github.com/kust1q/numa-aware-scheduler/pkg/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic/fake"
)

func TestUpdateTopologyCreate(t *testing.T) {
	scheme := runtime.NewScheme()
	dynClient := fake.NewSimpleDynamicClient(scheme)

	c := &Client{dynClient: dynClient}

	spec := &v1.NumaTopologySpec{
		NumaNodes: []v1.NumaNode{
			{ID: 0, CPUs: []int{0, 1}},
		},
	}

	err := c.UpdateTopology(context.Background(), "node1", spec)
	if err != nil {
		t.Fatalf("UpdateTopology returned error: %v", err)
	}

	// verify creation
	obj, err := dynClient.Resource(numaTopologyGVR).Get(context.Background(), "node1", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Expected object to be created: %v", err)
	}
	if obj.GetName() != "node1" {
		t.Errorf("expected node1, got %s", obj.GetName())
	}
}

func TestUpdateTopologyUpdate(t *testing.T) {
	scheme := runtime.NewScheme()

	existing := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "topology.numa-aware-scheduler.io/v1alpha1",
			"kind":       "NumaTopology",
			"metadata": map[string]interface{}{
				"name": "node1",
			},
			"spec": map[string]interface{}{
				"numaNodes": []interface{}{},
			},
		},
	}

	dynClient := fake.NewSimpleDynamicClient(scheme, existing)
	c := &Client{dynClient: dynClient}

	spec := &v1.NumaTopologySpec{
		NumaNodes: []v1.NumaNode{
			{ID: 0, CPUs: []int{0, 1}},
		},
	}

	err := c.UpdateTopology(context.Background(), "node1", spec)
	if err != nil {
		t.Fatalf("UpdateTopology returned error: %v", err)
	}

	// verify update
	obj, err := dynClient.Resource(numaTopologyGVR).Get(context.Background(), "node1", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Expected object to exist: %v", err)
	}

	specMap, _, _ := unstructured.NestedMap(obj.Object, "spec")
	nodes, _, _ := unstructured.NestedSlice(specMap, "numaNodes")
	if len(nodes) != 1 {
		t.Errorf("Expected 1 node, got %d", len(nodes))
	}
}
