package drivers

import (
	"errors"
)

type RackzonCoreClient struct {
	rack_client RackspaceCoreClient
	ec2_client  EC2CoreClient
	cache       *CredCache
}

func RackzonGetClient(project string, region string, cache_path string) (RackzonCoreClient, error) {
	c := RackzonCoreClient{}
	rack_client, err := RackspaceGetClient(project, region, cache_path)
	if err != nil {
		return c, err
	}
	c.rack_client = rack_client
	ec2_client, err := EC2GetClient(project, region, cache_path)
	if err != nil {
		return c, err
	}
	c.ec2_client = ec2_client
	return c, nil

}

func (c RackzonCoreClient) Run(project string, channel string, region string, size string, num int, block bool, cloud_config string, image string) error {
	return errors.New("not implemented")
}
func (c RackzonCoreClient) Terminate(project string) error {
	return errors.New("not implemented")
}
func (c RackzonCoreClient) List(project string) error {
	c.rack_client.List(project)
	c.ec2_client.List(project)
	return nil
}
