// Package main provides the entry point for the custom Kubernetes scheduler.
package main

import (
	"os"

	"github.com/kust1q/numa-aware-scheduler/internal/scheduler/plugins/numaware"
	"k8s.io/component-base/cli"
	"k8s.io/kubernetes/cmd/kube-scheduler/app"
)

func main() {
	command := app.NewSchedulerCommand(
		app.WithPlugin(numaware.Name, numaware.New),
	)

	code := cli.Run(command)
	os.Exit(code)
}
