package drivers

import (
	"errors"

	"github.com/polvi/coreup/drivers/do"
	"github.com/polvi/coreup/drivers/ec2"
	"github.com/polvi/coreup/drivers/gce"
	"github.com/polvi/coreup/drivers/rackspace"
)

func FromName(provider, project, region, cachePath string) (Client, error) {
	switch provider {
	case "ec2":
		return ec2.GetClient(project, region, cachePath)
	case "rackspace":
		return rackspace.GetClient(project, region, cachePath)
	case "google":
		return gce.GetClient(project, region, cachePath)
	case "do":
		return do.GetClient(project, region, cachePath)
	}
	return nil, errors.New("Unable to find provider")
}

type Client interface {
	Run(project string, channel string, size string, num int, block bool, cloud_config string, image string) error
	List(project string) error
	Terminate(project string) error
}
