package k8sclient

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/kust1q/numa-aware-scheduler/pkg/api/numatopology/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

var numaTopologyGVR = schema.GroupVersionResource{
	Group:    "topology.numa-aware-scheduler.io",
	Version:  "v1alpha1",
	Resource: "numatopologies",
}

type Client struct {
	dynClient dynamic.Interface
}

func New() (*Client, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		// Fallback for local testing (out-of-cluster) could be added here,
		// but standard agent runs in cluster.
		return nil, fmt.Errorf("failed to get in-cluster config: %w", err)
	}

	dynClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	return &Client{dynClient: dynClient}, nil
}

func (c *Client) UpdateTopology(ctx context.Context, nodeName string, spec *v1alpha1.NumaTopologySpec) error {
	resourceClient := c.dynClient.Resource(numaTopologyGVR)

	existing, errGet := resourceClient.Get(ctx, nodeName, metav1.GetOptions{})

	specMap, err := structToMap(spec)
	if err != nil {
		return err
	}

	if errGet == nil {
		existing.Object["spec"] = specMap
		_, err = resourceClient.Update(ctx, existing, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update NumaTopology for node %s: %w", nodeName, err)
		}
		return nil
	}

	newCR := &unstructured.Unstructured{
		Object: map[string]interface{}{"apiVersion": "topology.numa-aware-scheduler.io/v1alpha1",
			"kind": "NumaTopology",
			"metadata": map[string]interface{}{
				"name": nodeName,
			},
			"spec": specMap,
		},
	}

	_, err = resourceClient.Create(ctx, newCR, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create NumaTopology for node %s: %w", nodeName, err)
	}

	return nil
}

func structToMap(obj interface{}) (map[string]interface{}, error) {
	bytes, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	var res map[string]interface{}
	err = json.Unmarshal(bytes, &res)
	if err != nil {
		return nil, err
	}
	return res, nil
}
