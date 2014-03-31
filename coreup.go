package main

import (
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"

	"github.com/polvi/coreup/drivers"
)

type CoreClient interface {
	Run(project string, channel string, region string, size string, num int, cloud_config string) error
	Terminate(project string) error
}

func getClient(project string, region string) (CoreClient, error) {
	if project == "ec2" {
		return drivers.EC2GetClient(project, region)
	}
	return nil, errors.New("Unable to find provider")
}
func main() {
	var cloudConfigFlag string
	flag.StringVar(&cloudConfigFlag, "cloud-config", "", "local file, usually ./cloud-config.yml")
	var channelFlag string
	flag.StringVar(&channelFlag, "channel", "alpha", "CoreOS channel to use")
	var providerFlag string
	flag.StringVar(&providerFlag, "provider", "ec2", "cloud or provider to launch instance in")
	var regionFlag string
	flag.StringVar(&regionFlag, "region", "us-west-2", "region to launch instance in")
	var actionFlag string
	flag.StringVar(&actionFlag, "action", "run", "run, terminate, list")
	var sizeFlag string
	flag.StringVar(&sizeFlag, "size", "m1.medium", "size of instance")
	var projectFlag string
	user, _ := user.Current()
	flag.StringVar(&projectFlag, "project", "coreup-"+user.Username, "name for the group of servers in the same project")
	var numFlag int
	flag.IntVar(&numFlag, "num", 1, "number of instances to launch like this")
	flag.Parse()
	var cloud_config string
	if cloudConfigFlag != "" {
		data, err := ioutil.ReadFile(cloudConfigFlag)
		if err != nil {
			println("unable to read cloud-config")
		}
		cloud_config = string(data)
	}
	c, err := getClient(providerFlag, regionFlag)
	if err != nil {
		fmt.Println("error getting client", providerFlag, err)
		os.Exit(1)
	}
	switch actionFlag {
	case "run":
		err = c.Run(projectFlag, channelFlag, regionFlag, sizeFlag, numFlag, cloud_config)
		if err != nil {
			fmt.Println("error launching instances", err)
			os.Exit(1)
		}
	case "terminate":
		err = c.Terminate(projectFlag)
		if err != nil {
			fmt.Println("error terminating instances", err)
			os.Exit(1)
		}
	}
}
