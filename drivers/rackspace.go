package drivers

import (
	"encoding/base64"
	"fmt"
	"github.com/rackspace/gophercloud"
	"os"
)

type RackspaceCoreClient struct {
	client gophercloud.CloudServersProvider
}

func RackspaceGetClient(project string, region string) (RackspaceCoreClient, error) {
	c := RackspaceCoreClient{}
	ao := gophercloud.AuthOptions{
		Username:    os.ExpandEnv("$RACKSPACE_USER"),
		ApiKey:      os.ExpandEnv("$RACKSPACE_API_KEY"),
		AllowReauth: true,
	}
	access, err := gophercloud.Authenticate("rackspace-us", ao)
	if err != nil {
		fmt.Println("unable to auth rackspace", err)
		return c, err
	}
	ac, err := gophercloud.PopulateApi("rackspace")
	client, err := gophercloud.ServersApi(access, ac)
	c.client = client
	if err != nil {
		fmt.Println("unable to get rackspace client", err)
		return c, err
	}
	return c, nil

}

func (c RackspaceCoreClient) Run(project string, channel string, region string, size string, num int, cloud_config string) error {
	b := []byte(cloud_config)
	cc_b64 := base64.StdEncoding.EncodeToString(b)
	metadata := map[string]string{"coreup": project}
	ns := gophercloud.NewServer{
		Name:        project,
		Metadata:    metadata,
		ImageRef:    "6bdbd558-e66c-49cc-9ff3-126e7411f602",
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
