package rackspace

import (
	"encoding/base64"
	"fmt"

	"github.com/polvi/coreup/config"
	"github.com/polvi/coreup/Godeps/_workspace/src/github.com/rackspace/gophercloud"
)

type Client struct {
	client gophercloud.CloudServersProvider
	cache  *config.CredCache
	region string
}

const (
	defaultImage  = "6bdbd558-e66c-49cc-9ff3-126e7411f602"
	defaultRegion = "ORD"
)

func GetClient(project string, region string, cache_path string) (Client, error) {
	c := Client{}
	cache, err := config.LoadCredCache(cache_path)
	if err != nil {
		fmt.Println("unable to get cache")
		return c, err
	}
	c.cache = cache
	if region == "" {
		region = defaultRegion
	}
	c.region = region
	if c.cache.Rackspace.User == "" || c.cache.Rackspace.APIKey == "" {
		var rack_user string
		var rack_pass string
		fmt.Printf("rackspace user: ")
		_, err = fmt.Scanf("%s", &rack_user)
		if err != nil {
			return c, err
		}
		fmt.Printf("rackspace api key: ")
		_, err = fmt.Scanf("%s", &rack_pass)
		if err != nil {
			return c, err
		}
		c.cache.Rackspace.User = rack_user
		c.cache.Rackspace.APIKey = rack_pass
		c.cache.Save()
		if err != nil {
			return c, err
		}
	}
	ao := gophercloud.AuthOptions{
		Username:    c.cache.Rackspace.User,
		ApiKey:      c.cache.Rackspace.APIKey,
		AllowReauth: true,
	}
	access, err := gophercloud.Authenticate("rackspace-us", ao)
	if err != nil {
		c.cache.Rackspace.User = ""
		c.cache.Rackspace.APIKey = ""
		c.cache.Save()
		return c, err
	}
	ac, err := gophercloud.PopulateApi("rackspace")
	if err != nil {
		return c, err
	}
	client, err := gophercloud.ServersApi(access, ac)
	if err != nil {
		return c, err
	}
	c.client = client
	return c, nil
}

func (c Client) Run(project string, channel string, size string, num int, block bool, cloud_config string, image string) error {
	b := []byte(cloud_config)
	cc_b64 := base64.StdEncoding.EncodeToString(b)
	metadata := map[string]string{"coreup": project}
	if image == "" {
		image = defaultImage
	}
	ns := gophercloud.NewServer{
		Name:        project,
		Metadata:    metadata,
		ImageRef:    image,
		FlavorRef:   size,
		ConfigDrive: true,
		UserData:    cc_b64,
	}

	_, err := c.client.CreateServer(ns)
	if err != nil {
		fmt.Println("unable to create server", err)
		return err
	}
	return nil
}
func (c Client) serversByProject(project string) ([]gophercloud.Server, error) {
	matches := make([]gophercloud.Server, 0)
	servers, err := c.client.ListServers()
	if err != nil {
		fmt.Println("unable to list servers", err)
		return matches, err
	}
	for _, s := range servers {
		if v, present := s.Metadata["coreup"]; present && v == project {
			matches = append(matches, s)
		}
	}
	return matches, err

}
func (c Client) Terminate(project string) error {
	servers, err := c.serversByProject(project)
	for _, s := range servers {
		err = c.client.DeleteServerById(s.Id)
		if err != nil {
			fmt.Printf("error deleting %s: %s", s.Name, err)
		}
	}
	return nil
}
func (c Client) List(project string) error {
	servers, err := c.serversByProject(project)
	if err != nil {
		fmt.Println("unable to list servers", err)
		return err
	}
	for _, s := range servers {
		fmt.Println(s.AccessIPv4)
	}
	return err
}
