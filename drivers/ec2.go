package drivers

import (
	"fmt"
	"github.com/mitchellh/goamz/aws"
	"github.com/mitchellh/goamz/ec2"
	"io/ioutil"
	"net/http"
	"strings"
)

type EC2CoreClient struct {
	client *ec2.EC2
}

func EC2GetClient(project string, region string) (EC2CoreClient, error) {
	c := EC2CoreClient{}
	auth, err := aws.EnvAuth()
	if err != nil {
		println("unable to get aws client")
		return c, err
	}
	client := ec2.New(auth, aws.Regions[region])
	c.client = client
	return c, nil

}
func getEc2AmiUrl(channel string) string {
	return fmt.Sprintf("http://storage.core-os.net/coreos/amd64-usr/%s/coreos_production_ami_all.txt", channel)
}
func ec2GetAmis(url string) (map[string]string, error) {
	resp, err := http.Get(url)
	defer resp.Body.Close()
	if err != nil {
		return nil, err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	imgs := strings.Split(string(body), "|")
	ret := make(map[string]string)
	for _, img := range imgs {
		region_ami := strings.Split(img, "=")
		ret[region_ami[0]] = region_ami[1]
	}
	return ret, err
}
func ec2GetSecurityGroup(client *ec2.EC2, project string) ec2.SecurityGroup {
	sg := ec2.SecurityGroup{
		Name:        project,
		Description: "automatically created by coreup",
	}
	_, err := client.CreateSecurityGroup(sg)
	if err != nil {
		// non-fatal, as it is probably already created
	}
	perms := []ec2.IPPerm{ec2.IPPerm{Protocol: "tcp", FromPort: 1, ToPort: 65000, SourceIPs: []string{"0.0.0.0/0"}}}
	_, err = client.AuthorizeSecurityGroup(sg, perms)
	if err != nil {
		// non-fatal, as it is probably already authorized
	}
	return sg
}
func (c EC2CoreClient) Run(project string, channel string, region string, size string, num int, cloud_config string) error {
	amis, _ := ec2GetAmis(getEc2AmiUrl(channel))
	sg := ec2GetSecurityGroup(c.client, project)
	options := ec2.RunInstances{
		ImageId:        amis[region],
		MinCount:       num,
		MaxCount:       num,
		UserData:       []byte(cloud_config),
		SecurityGroups: []ec2.SecurityGroup{sg},
		InstanceType:   size,
	}
	resp, err := c.client.RunInstances(&options)
	if err != nil {
		fmt.Println("could not create instances", err)
		return err
	}
	ids := make([]string, 0)
	for _, instance := range resp.Instances {
		ids = append(ids, instance.InstanceId)
	}
	_, err = c.client.CreateTags(ids, []ec2.Tag{ec2.Tag{Key: "Name", Value: project}})
	if err != nil {
		fmt.Println("could not tag group", err)
		return err
	}
	return nil
}

func (c EC2CoreClient) Terminate(project string) error {
	filter := ec2.NewFilter()
	filter.Add("tag:Name", project)
	filter.Add("instance-state-name", "running")
	resp, err := c.client.Instances(nil, filter)
	if err != nil {
		fmt.Println("could get instances", err)
	}
	ids := make([]string, 0)
	for _, res := range resp.Reservations {
		for _, instance := range res.Instances {
			ids = append(ids, instance.InstanceId)
		}
	}
	if len(ids) > 0 {
		_, err = c.client.TerminateInstances(ids)
		if err != nil {
			fmt.Println("could get terminate instances", err)
			return err
		}
	}
	sg := ec2.SecurityGroup{
		Name: project,
	}
	_, err = c.client.DeleteSecurityGroup(sg)
	if err != nil {
		// will fail if the machines are not terminated
	}
	return nil
}
