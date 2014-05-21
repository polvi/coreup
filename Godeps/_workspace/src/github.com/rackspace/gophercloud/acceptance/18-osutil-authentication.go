package main

import (
	"github.com/polvi/coreup/Godeps/_workspace/src/github.com/rackspace/gophercloud"
	"github.com/polvi/coreup/Godeps/_workspace/src/github.com/rackspace/gophercloud/osutil"
)

func main() {
	provider, authOptions, err := osutil.AuthOptions()
	if err != nil {
		panic(err)
	}
	_, err = gophercloud.Authenticate(provider, authOptions)
	if err != nil {
		panic(err)
	}
}
