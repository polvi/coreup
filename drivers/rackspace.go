package drivers

import (
	"encoding/base64"
	"fmt"
	"github.com/rackspace/gophercloud"
)

type RackspaceCoreClient struct {
	client gophercloud.CloudServersProvider
	cache  *CredCache
}

const (
	defaultImage = "6bdbd558-e66c-49cc-9ff3-126e7411f602"
)

func RackspaceGetClient(project string, region string, cache_path string) (RackspaceCoreClient, error) {
	c := RackspaceCoreClient{}
	cache, err := LoadCredCache(cache_path)
	if err != nil {
		fmt.Println("unable to get cache")
		return c, err
	}
	c.cache = cache
	if c.cache.RackspaceUser == "" || c.cache.RackspaceAPIKey == "" {
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
		c.cache.RackspaceUser = rack_user
		c.cache.RackspaceAPIKey = rack_pass
		c.cache.Save()
		if err != nil {
			return c, err
		}
	}
	ao := gophercloud.AuthOptions{
		Username:    c.cache.RackspaceUser,
		ApiKey:      c.cache.RackspaceAPIKey,
		AllowReauth: true,
	}
	access, err := gophercloud.Authenticate("rackspace-us", ao)
	if err != nil {
		c.cache.RackspaceUser = ""
		c.cache.RackspaceAPIKey = ""
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

func (c RackspaceCoreClient) Run(project string, channel string, region string, size string, num int, block bool, cloud_config string, image string) error {
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
func (c RackspaceCoreClient) serversByProject(project string) ([]gophercloud.Server, error) {
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
func (c RackspaceCoreClient) Terminate(project string) error {
	servers, err := c.serversByProject(project)
	for _, s := range servers {
		err = c.client.DeleteServerById(s.Id)
		if err != nil {
			fmt.Printf("error deleting %s: %s", s.Name, err)
		}
	}
	return nil
}
func (c RackspaceCoreClient) List(project string) error {
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
