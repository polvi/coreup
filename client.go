package main

import (
	"errors"

	"github.com/polvi/coreup/drivers"
)

type CoreClient interface {
	Run(project string, channel string, size string, num int, block bool, cloud_config string, image string) error
	List(project string) error
	Terminate(project string) error
}

func getClient(project, provider, region, cachePath string) (CoreClient, error) {
	switch provider {
	case "ec2":
		return drivers.EC2GetClient(project, region, cachePath)
	case "rackspace":
		return drivers.RackspaceGetClient(project, region, cachePath)
	case "google":
		return drivers.GCEGetClient(project, region, cachePath)
	}
	return nil, errors.New("Unable to find provider")
}
