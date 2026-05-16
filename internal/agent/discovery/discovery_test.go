package discovery

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseCPUList(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected []int
		wantErr  bool
	}{
		{"empty", "", []int{}, false},
		{"single", "0", []int{0}, false},
		{"list", "0,1,2", []int{0, 1, 2}, false},
		{"range", "0-3", []int{0, 1, 2, 3}, false},
		{"mixed", "0,2-4,6", []int{0, 2, 3, 4, 6}, false},
		{"invalid", "0-a", nil, true},
	}

	dir := t.TempDir()
	file := filepath.Join(dir, "cpulist")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := os.WriteFile(file, []byte(tt.content), 0600)
			if err != nil {
				t.Fatalf("failed to write temp file: %v", err)
			}

			got, err := parseCPUList(file)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseCPUList() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("parseCPUList() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestParseMemInfo(t *testing.T) {
	content := `Node 0 MemTotal:       16384 kB
Node 0 MemFree:        10000 kB`

	dir := t.TempDir()
	file := filepath.Join(dir, "meminfo")
	err := os.WriteFile(file, []byte(content), 0600)
	if err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	got, err := parseMemInfo(file)
	if err != nil {
		t.Errorf("parseMemInfo() error = %v", err)
	}
	expected := int64(16384 * 1024)
	if got != expected {
		t.Errorf("parseMemInfo() = %v, want %v", got, expected)
	}
}

func TestDiscover(t *testing.T) {
	dir := t.TempDir()

	node0 := filepath.Join(dir, "node0")
	if err := os.MkdirAll(node0, 0750); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(node0, "cpulist"), []byte("0-3\n"), 0600); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(node0, "meminfo"), []byte("Node 0 MemTotal:       16384 kB\n"), 0600); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	node1 := filepath.Join(dir, "node1")
	if err := os.MkdirAll(node1, 0750); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(node1, "cpulist"), []byte("4-7\n"), 0600); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(node1, "meminfo"), []byte("Node 1 MemTotal:       16384 kB\n"), 0600); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	// dummy dir
	if err := os.MkdirAll(filepath.Join(dir, "notanode"), 0750); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	orig := sysfsNodePath
	sysfsNodePath = dir
	defer func() { sysfsNodePath = orig }()

	spec, err := Discover()
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}

	if len(spec.NumaNodes) != 2 {
		t.Errorf("expected 2 numa nodes, got %d", len(spec.NumaNodes))
	}
}
