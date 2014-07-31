package main

import (
	"fmt"
	"io/ioutil"
	"os"
)

var (
	cloudConfigPath string
	channel         string
	provider        string
	action          string
	size            string
	image           string
	num             int
	block           bool

	cmdRun = &Command{
		Name:        "run",
		Summary:     "run",
		Description: `run`,
		Run:         runRun,
	}
)

func init() {
	cmdRun.Flags.StringVar(&cloudConfigPath, "cloud-config", "./cloud-config.yml", "local file, usually ./cloud-config.yml")
	cmdRun.Flags.StringVar(&channel, "channel", "alpha", "CoreOS channel to use")
	cmdRun.Flags.StringVar(&provider, "provider", "ec2", "cloud or provider to launch instance in")
	cmdRun.Flags.StringVar(&size, "size", "t2.micro", "size of instance")
	cmdRun.Flags.IntVar(&num, "num", 1, "number of instances to launch like this")
	cmdRun.Flags.BoolVar(&block, "block-until-ready", true, "tell run commands to wait until machines are up to return")
	cmdRun.Flags.StringVar(&image, "image", "", "image name (default to fetching from core-os.net)")
}

func runRun(args []string) (exit int) {
	data, err := ioutil.ReadFile(cloudConfigPath)
	if err != nil {
		fmt.Println("unable to", err)
		os.Exit(1)
	}
	body := string(data)

	err = client.Run(globalFlags.project, channel, size, num, block, body, image)
	if err != nil {
		fmt.Println("error launching instances", err)
		os.Exit(1)
	}

	return 0
}
