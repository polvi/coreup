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
	Run(project string, channel string, region string, size string, num int, block bool, cloud_config string, image string) error
	List(project string) error
	Terminate(project string) error
}

func getClient(provider string, region string) (CoreClient, error) {
	switch provider {
	case "ec2":
		return drivers.EC2GetClient(project, region, cache_path)
	case "rackspace":
		return drivers.RackspaceGetClient(project, "", cache_path)
	case "google":
		return drivers.GCEGetClient(project, region, cache_path)
	}
	return nil, errors.New("Unable to find provider")
}

var (
	cloudConfig = flag.String("cloud-config", "./cloud-config.yml", "local file, usually ./cloud-config.yml")
	channel     = flag.String("channel", "alpha", "CoreOS channel to use")
	provider    = flag.String("provider", "ec2", "cloud or provider to launch instance in")
	region      = flag.String("region", "", "region to launch instance in")
	action      = flag.String("action", "run", "run, terminate, list")
	size        = flag.String("size", "m1.medium", "size of instance")
	num         = flag.Int("num", 1, "number of instances to launch like this")
	block       = flag.Bool("block-until-ready", true, "tell run commands to wait until machines are up to return")

	project    string
	cache_path string
	image      string
)

func init() {
	usr, _ := user.Current()
	flag.StringVar(&project, "project", "coreup-"+usr.Username, "name for the group of servers in the same project")
	flag.StringVar(&cache_path, "cred-cache", usr.HomeDir+"/.coreup/cred-cache.json", "location to store credential tokens")
	flag.StringVar(&image, "image", "", "image name (default to fetching from core-os.net)")
}

func main() {
	flag.Parse()

	var cloud_config string
	if *cloudConfig != "" && *action == "run" {
		data, err := ioutil.ReadFile(*cloudConfig)
		if err != nil {
			fmt.Println("unable to", err)
			os.Exit(1)
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
		err = c.Run(project, *channel, *region, *size, *num, *block, cloud_config, image)
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
