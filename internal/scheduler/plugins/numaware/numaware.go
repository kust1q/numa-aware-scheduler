package numaware

import (
	"context"
	"encoding/json"
	"fmt"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/kubernetes/pkg/scheduler/framework"

	"github.com/kust1q/numa-aware-scheduler/pkg/api/numatopology/v1alpha1"
)

const (
	Name = "NumaAwarePlacement"
)

var numaTopologyGVR = schema.GroupVersionResource{
	Group:    "topology.numa-aware-scheduler.io",
	Version:  "v1alpha1",
	Resource: "numatopologies",
}

type NumaAware struct {
	handle    framework.Handle
	dynClient dynamic.Interface
}

var _ framework.FilterPlugin = &NumaAware{}
var _ framework.ScorePlugin = &NumaAware{}

// Name returns name of the plugin.
func (pl *NumaAware) Name() string {
	return Name
}

// New initializes a new plugin and returns it.
func New(_ context.Context, obj runtime.Object, h framework.Handle) (framework.Plugin, error) {
	dynClient, err := dynamic.NewForConfig(h.KubeConfig())
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	return &NumaAware{
		handle:    h,
		dynClient: dynClient,
	}, nil
}

func (pl *NumaAware) getTopology(ctx context.Context, nodeName string) (*v1alpha1.NumaTopologySpec, error) {
	unstruct, err := pl.dynClient.Resource(numaTopologyGVR).Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	specMap, found, err := unstructured.NestedMap(unstruct.Object, "spec")
	if !found || err != nil {
		return nil, fmt.Errorf("spec not found in NumaTopology")
	}

	specJSON, err := json.Marshal(specMap)
	if err != nil {
		return nil, err
	}

	var spec v1alpha1.NumaTopologySpec
	if err := json.Unmarshal(specJSON, &spec); err != nil {
		return nil, err
	}

	return &spec, nil
}

func getPodCPURequest(pod *v1.Pod) int64 {
	var totalCPU int64 = 0
	for _, container := range pod.Spec.Containers {
		if cpuReq := container.Resources.Requests.Cpu(); cpuReq != nil {
			totalCPU += cpuReq.MilliValue()
		}
	}
	// Convert milliCPUs to whole CPUs (ceiling)
	return (totalCPU + 999) / 1000
}

// Filter invoked at the filter extension point.
func (pl *NumaAware) Filter(ctx context.Context, state *framework.CycleState, pod *v1.Pod, nodeInfo *framework.NodeInfo) *framework.Status {
	node := nodeInfo.Node()
	if node == nil {
		return framework.NewStatus(framework.Error, "node not found")
	}

	cpuReq := getPodCPURequest(pod)
	if cpuReq == 0 {
		return framework.NewStatus(framework.Success, "")
	}

	topo, err := pl.getTopology(ctx, node.Name)
	if err != nil {
		// If topology info is missing, assume it's not a NUMA node and pass it.
		return framework.NewStatus(framework.Success, "")
	}

	// Filter rule: Pod must fit within at least ONE single NUMA node.
	canFit := false
	for _, numaNode := range topo.NumaNodes {
		if int64(len(numaNode.CPUs)) >= cpuReq {
			canFit = true
			break
		}
	}

	if !canFit {
		return framework.NewStatus(framework.Unschedulable, "Not enough CPUs on any single NUMA node")
	}

	return framework.NewStatus(framework.Success, "")
}

// Score invoked at the score extension point.
func (pl *NumaAware) Score(ctx context.Context, state *framework.CycleState, p *v1.Pod, nodeName string) (int64, *framework.Status) {
	cpuReq := getPodCPURequest(p)
	if cpuReq == 0 {
		return framework.MinNodeScore, framework.NewStatus(framework.Success, "")
	}

	topo, err := pl.getTopology(ctx, nodeName)
	if err != nil {
		return framework.MinNodeScore, framework.NewStatus(framework.Success, "")
	}

	var bestScore int64 = framework.MinNodeScore
	for _, numaNode := range topo.NumaNodes {
		availCPUs := int64(len(numaNode.CPUs))
		if availCPUs >= cpuReq {
			remaining := availCPUs - cpuReq
			score := framework.MaxNodeScore - remaining
			if score < framework.MinNodeScore {
				score = framework.MinNodeScore
			}
			if score > framework.MaxNodeScore {
				score = framework.MaxNodeScore
			}

			if score > bestScore {
				bestScore = score
			}
		}
	}

	return bestScore, framework.NewStatus(framework.Success, "")
}

// ScoreExtensions of the Score plugin.
func (pl *NumaAware) ScoreExtensions() framework.ScoreExtensions {
	return nil
}
