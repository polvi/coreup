package main

import (
	"fmt"
)

var (
	cmdTerminate = &Command{
		Name:        "terminate",
		Summary:     "Terminate machines in this project",
		Description: `Terminate the hostnames of the machines in this project. The hostnames will be newline separated and have public DNS names..`,
		Run:         runTerminate,
	}
)

func runTerminate(args []string) (exit int) {
	err := client.Terminate(globalFlags.project)
	if err != nil {
		fmt.Println("error terminateing instances", err)
		return 1
	}

	return 0
}
