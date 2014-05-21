package main

import (
	"fmt"
	"github.com/polvi/coreup/Godeps/_workspace/src/github.com/rackspace/gophercloud"
	"os"
	"strings"
)

func main() {
	provider, username, _, apiKey := getCredentials()

	if !strings.Contains(provider, "rackspace") {
		fmt.Fprintf(os.Stdout, "Skipping test because provider doesn't support API_KEYs\n")
		return
	}

	_, err := gophercloud.Authenticate(
		provider,
		gophercloud.AuthOptions{
			Username: username,
			ApiKey:   apiKey,
		},
	)
	if err != nil {
		panic(err)
	}
}
