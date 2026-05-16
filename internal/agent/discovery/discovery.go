package discovery

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/kust1q/numa-aware-scheduler/pkg/api/numatopology/v1alpha1"
)

var sysfsNodePath = "/sys/devices/system/node"

// Discover reads NUMA topology from sysfs
func Discover() (*v1alpha1.NumaTopologySpec, error) {
	spec := &v1alpha1.NumaTopologySpec{}

	files, err := os.ReadDir(sysfsNodePath)
	if err != nil {
		if os.IsNotExist(err) {
			// No NUMA support
			return spec, nil
		}
		return nil, err
	}

	for _, f := range files {
		if !f.IsDir() || !strings.HasPrefix(f.Name(), "node") {
			continue
		}

		nodeIDStr := strings.TrimPrefix(f.Name(), "node")
		nodeID, err := strconv.Atoi(nodeIDStr)
		if err != nil {
			continue // Not a node directory
		}

		cpulistPath := filepath.Join(sysfsNodePath, f.Name(), "cpulist")
		meminfoPath := filepath.Join(sysfsNodePath, f.Name(), "meminfo")

		cpus, err := parseCPUList(cpulistPath)
		if err != nil {
			return nil, fmt.Errorf("failed to parse cpulist for node %d: %w", nodeID, err)
		}

		mem, err := parseMemInfo(meminfoPath)
		if err != nil {
			return nil, fmt.Errorf("failed to parse meminfo for node %d: %w", nodeID, err)
		}

		spec.NumaNodes = append(spec.NumaNodes, v1alpha1.NumaNode{
			ID:     nodeID,
			CPUs:   cpus,
			Memory: mem,
		})
	}

	return spec, nil
}

func parseCPUList(path string) ([]int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	content := strings.TrimSpace(string(data))
	if content == "" {
		return []int{}, nil
	}

	var cpus []int
	parts := strings.Split(content, ",")
	for _, part := range parts {
		if strings.Contains(part, "-") {
			rangeParts := strings.Split(part, "-")
			if len(rangeParts) != 2 {
				return nil, fmt.Errorf("invalid range: %s", part)
			}
			start, err := strconv.Atoi(rangeParts[0])
			if err != nil {
				return nil, err
			}
			end, err := strconv.Atoi(rangeParts[1])
			if err != nil {
				return nil, err
			}
			for i := start; i <= end; i++ {
				cpus = append(cpus, i)
			}
		} else {
			cpu, err := strconv.Atoi(part)
			if err != nil {
				return nil, err
			}
			cpus = append(cpus, cpu)
		}
	}
	return cpus, nil
}

func parseMemInfo(path string) (int64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.Contains(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) >= 4 {
				kb, err := strconv.ParseInt(fields[3], 10, 64)
				if err != nil {
					return 0, err
				}
				return kb * 1024, nil
			}
		}
	}
	return 0, fmt.Errorf("MemTotal not found in %s", path)
}
