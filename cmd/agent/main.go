// Package main provides the entry point for the NUMA topology discovery agent.
package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/kust1q/numa-aware-scheduler/internal/agent/discovery"
	"github.com/kust1q/numa-aware-scheduler/internal/agent/k8sclient"
)

func main() {
	log.Println("Starting NUMA topology discovery agent...")

	nodeName := os.Getenv("NODE_NAME")
	if nodeName == "" {
		log.Fatalf("NODE_NAME environment variable is required")
	}

	client, err := k8sclient.New()
	if err != nil {
		log.Fatalf("Failed to initialize k8s client: %v", err)
	}

	for {
		spec, err := discovery.Discover()
		if err != nil {
			log.Printf("Error discovering NUMA topology: %v", err)
		} else {
			log.Printf("Discovered %d NUMA nodes", len(spec.NumaNodes))
			err = client.UpdateTopology(context.Background(), nodeName, spec)
			if err != nil {
				log.Printf("Error updating NUMA topology in k8s: %v", err)
			} else {
				log.Printf("Successfully updated NUMA topology for node %s", nodeName)
			}
		}

		time.Sleep(1 * time.Minute)
	}
}
