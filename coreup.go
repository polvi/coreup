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
	List(project string) error
	Terminate(project string) error
}

func getClient(provider string, region string) (CoreClient, error) {
	switch provider {
	case "ec2":
		return drivers.EC2GetClient(project, region)
	case "rackspace":
		return drivers.RackspaceGetClient(project, "", cache_path)
	}
	return nil, errors.New("Unable to find provider")
}

var (
	cloudConfig = flag.String("cloud-config", "", "local file, usually ./cloud-config.yml")
	channel     = flag.String("channel", "alpha", "CoreOS channel to use")
	provider    = flag.String("provider", "ec2", "cloud or provider to launch instance in")
	region      = flag.String("region", "us-west-2", "region to launch instance in")
	action      = flag.String("action", "run", "run, terminate, list")
	size        = flag.String("size", "m1.medium", "size of instance")
	num         = flag.Int("num", 1, "number of instances to launch like this")

	project    string
	cache_path string
)

func init() {
	usr, _ := user.Current()
	flag.StringVar(&project, "project", "coreup-"+usr.Username, "name for the group of servers in the same project")
	flag.StringVar(&cache_path, "cred-cache", usr.HomeDir+"/.coreup/cred-cache.json", "location to store credential tokens")
}

func main() {
	flag.Parse()

	var cloud_config string
	if *cloudConfig != "" {
		data, err := ioutil.ReadFile(*cloudConfig)
		if err != nil {
			fmt.Println("unable to read cloud-config", err)
		}
		cloud_config = string(data)
	}
	c, err := getClient(*provider, *region)
	if err != nil {
		fmt.Println("error getting client", *provider, err)
		os.Exit(1)
	}
	switch *action {
	case "run":
		err = c.Run(project, *channel, *region, *size, *num, cloud_config)
		if err != nil {
			fmt.Println("error launching instances", err)
			os.Exit(1)
		}
	case "terminate":
		err = c.Terminate(project)
		if err != nil {
			fmt.Println("error terminating instances", err)
			os.Exit(1)
		}
	case "list":
		err = c.List(project)
		if err != nil {
			fmt.Println("error listing instances", err)
			os.Exit(1)
		}
	}
}
