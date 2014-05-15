package ec2

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"code.google.com/p/goauth2/oauth"
	"github.com/polvi/coreup/drivers/gce"
	"github.com/polvi/goamz/aws"
	"github.com/polvi/goamz/ec2"
	"github.com/polvi/goamz/sts"

	"github.com/polvi/coreup/config"
)

const defaultRegion = "us-west-2"

func authAWSFromToken(token *oauth.Token, arn string) (*config.ExpiringAuth, error) {
	// all regions have the same sts endpoint, so we just use us-east-1
	client := sts.New(aws.Regions["us-east-1"])
	duration := 3600 // seconds
	expiry := time.Now().Add(time.Duration(duration) * time.Second)
	// we use the token expiry as the client id to avoid another
	// call to google to fetch an email or something
	if _, ok := token.Extra["id_token"]; !ok {
		return nil, errors.New("unable to find id_token")
	}
	resp, err := client.AssumeRoleWithWebIdentity(
		duration,
		"",
		"",
		arn,
		strconv.Itoa(int(token.Expiry.Unix())),
		token.Extra["id_token"])
	if err != nil {
		return nil, err
	}
	return &config.ExpiringAuth{
		Auth: aws.Auth{
			AccessKey: resp.Credentials.AccessKeyId,
			SecretKey: resp.Credentials.SecretAccessKey,
			Token:     resp.Credentials.SessionToken,
		},
		Expiry: expiry,
	}, nil
}

type Client struct {
	client *ec2.EC2
	cache  *config.CredCache
	region string
}

func GetClient(project string, region string, cache_path string) (Client, error) {
	// this will cause the google cache to be populated
	gce.GetClient(project, region, cache_path)
	c := Client{}
	cache, err := config.LoadCredCache(cache_path)
	if err != nil {
		return c, err
	}
	c.cache = cache
	if region == "" {
		region = defaultRegion
	}
	c.region = region
	if cache.AWS.RoleARN == "" {
		var arn string
		fmt.Printf("amazon role arn: ")
		_, err = fmt.Scanf("%s", &arn)
		if err != nil {
			return c, err
		}
		c.cache.AWS.RoleARN = strings.TrimSpace(arn)
		c.cache.Save()
	}
	if c.cache.AWS.Token.Expiry.Before(time.Now()) {
		auth, err := authAWSFromToken(&c.cache.GCE.Token, c.cache.AWS.RoleARN)
		if err != nil {
			return c, err
		}
		c.cache.AWS.Token = *auth
		c.cache.Save()
	}
	if _, ok := aws.Regions[c.region]; !ok {
		return c, errors.New("could not find region " + c.region)
	}
	c.client = ec2.New(c.cache.AWS.Token.Auth, aws.Regions[c.region])
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

func (c Client) Run(project string, channel string, size string, num int, block bool, cloud_config string, image string) error {
	ami := image
	if image == "" {
		amis, err := ec2GetAmis(getEc2AmiUrl(channel))
		if err != nil {
			return err
		}
		a, ok := amis[c.region]
		if !ok {
			return errors.New("could not find region " + c.region)
		}
		ami = a
	}
	sg := ec2GetSecurityGroup(c.client, project)
	options := ec2.RunInstances{
		ImageId:        ami,
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
	tags := []ec2.Tag{
		// for convenience
		ec2.Tag{Key: "Name", Value: project},
		// used for listing and terminating instances in this project
		ec2.Tag{Key: "coreup-project", Value: project},
	}
	_, err = c.client.CreateTags(ids, tags)
	if err != nil {
		fmt.Println("could not tag group", err)
		return err
	}
	if block {
		total_instances := len(resp.Instances)
		var running []ec2.Instance
		for {
			running, err = c.serversByProject(project)
			if err != nil {
				return err
			}
			if len(running) == total_instances {
				break
			}
			time.Sleep(1 * time.Second)
		}
		running_ips := []string{}
		for _, inst := range running {
			running_ips = append(running_ips, inst.DNSName)
		}
		blockUntilSSH(running_ips)
	}
	return nil
}

func blockUntilSSH(servers []string) {
	var wg sync.WaitGroup
	wg.Add(len(servers))
	for _, ip := range servers {
		go func(ip string) {
			for {
				_, err := net.DialTimeout("tcp", ip+":22", 400*time.Millisecond)
				if err == nil {
					fmt.Println(ip)
					wg.Done()
					return
				}
			}

		}(ip)
	}
	wg.Wait()
}

func (c Client) serversByProject(project string) ([]ec2.Instance, error) {
	filter := ec2.NewFilter()
	filter.Add("tag:coreup-project", project)
	filter.Add("instance-state-name", "running")
	resp, err := c.client.Instances(nil, filter)
	if err != nil {
		return []ec2.Instance{}, err
	}
	instances := make([]ec2.Instance, 0)
	for _, res := range resp.Reservations {
		for _, instance := range res.Instances {
			instances = append(instances, instance)
		}
	}
	return instances, nil

}

func (c Client) Terminate(project string) error {
	instances, err := c.serversByProject(project)
	if err != nil {
		return err
	}
	// goamz requires a list of instance ids
	ids := make([]string, 0)
	for _, instance := range instances {
		ids = append(ids, instance.InstanceId)
	}
	_, err = c.client.TerminateInstances(ids)
	if err != nil {
		fmt.Println("could get terminate instances", err)
		return err
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

func (c Client) List(project string) error {
	instances, err := c.serversByProject(project)
	if err != nil {
		return err
	}
	for _, instance := range instances {
		fmt.Println(instance.DNSName)
	}
	return nil
}
