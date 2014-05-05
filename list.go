package main

import (
	"fmt"
)

var (
	cmdList = &Command{
		Name:        "list",
		Summary:     "List machines in this project",
		Description: `List the hostnames of the machines in this project. The hostnames will be newline separated and have public DNS names..`,
		Run:         runList,
	}
)

func runList(args []string) (exit int) {
	err := client.List(globalFlags.project)
	if err != nil {
		fmt.Println("error listing instances", err)
		return 1
	}

	return 0
}
