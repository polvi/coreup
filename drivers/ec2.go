package drivers

import (
	"code.google.com/p/goauth2/oauth"
	"encoding/json"
	"fmt"
	"github.com/polvi/goamz/aws"
	"github.com/polvi/goamz/ec2"
	"github.com/polvi/goamz/sts"
	"github.com/skratchdot/open-golang/open"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"time"
)

var oauthCfg = &oauth.Config{
	AuthURL:     "https://accounts.google.com/o/oauth2/auth",
	TokenURL:    "https://accounts.google.com/o/oauth2/token",
	RedirectURL: "http://localhost:8016/oauth2callback",
	Scope:       "https://www.googleapis.com/auth/userinfo.email",
}

const profileInfoURL = "https://www.googleapis.com/oauth2/v1/userinfo?alt=json"

var assumeRoleARN string

type GoogleEmail struct {
	Id            string `json:"id"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
}

func handleOAuth2Callback(w http.ResponseWriter, r *http.Request) (aws.Auth, error) {
	auth := aws.Auth{}

	code := r.FormValue("code")

	t := &oauth.Transport{Config: oauthCfg}

	// Exchange the received code for a token
	token, _ := t.Exchange(code)

	//now get user data based on the Transport which has the token
	resp, err := t.Client().Get(profileInfoURL)
	if err != nil {
		return auth, err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return auth, err
	}
	var goog GoogleEmail
	err = json.Unmarshal(body, &goog)
	if err != nil {
		return auth, err
	}

	client := sts.New(aws.Regions["us-east-1"])
	aws_resp, err := client.AssumeRoleWithWebIdentity(3600, "", "", assumeRoleARN, goog.Email, token.Extra["id_token"])
	if err != nil {
		return auth, err
	}
	auth = aws.Auth{
		AccessKey: aws_resp.Credentials.AccessKeyId,
		SecretKey: aws_resp.Credentials.SecretAccessKey,
		Token:     aws_resp.Credentials.SessionToken,
	}
	if err != nil {
		return auth, err
	}
	return auth, nil
}

func authFromOAuth() (aws.Auth, error) {
	type authres struct {
		auth aws.Auth
		err  error
	}

	l, err := net.Listen("tcp", "localhost:8016")
	defer l.Close()
	if err != nil {
		return aws.Auth{}, err
	}
	url := oauthCfg.AuthCodeURL("")
	open.Run(url)
	ch := make(chan *authres, 1)
	f := func(w http.ResponseWriter, r *http.Request) {
		a, err := handleOAuth2Callback(w, r)
		ch <- &authres{a, err}
	}
	go http.Serve(l, http.HandlerFunc(f))
	r := <-ch
	return r.auth, r.err
}

type EC2CoreClient struct {
	client *ec2.EC2
	cache  *CredCache
}

func EC2GetClient(project string, region string, cache_path string) (EC2CoreClient, error) {
	c := EC2CoreClient{}
	cache, err := LoadCredCache(cache_path)
	if err != nil {
		return c, err
	}
	c.cache = cache
	if c.cache.AWSAccessKey == "" || c.cache.AWSSecretKey == "" {
		if cache.GoogSSOClientID == "" || cache.GoogSSOClientSecret == "" {
			var client_id string
			var client_secret string
			fmt.Printf("google client id: ")
			_, err = fmt.Scanf("%s", &client_id)
			if err != nil {
				return c, err
			}
			c.cache.GoogSSOClientID = strings.TrimSpace(client_id)
			fmt.Printf("google client secret: ")
			_, err = fmt.Scanf("%s", &client_secret)
			if err != nil {
				return c, err
			}
			c.cache.GoogSSOClientSecret = strings.TrimSpace(client_secret)
			if err != nil {
				return c, err
			}
			if cache.AWSRoleARN == "" {
				var arn string
				fmt.Printf("amazon role arn: ")
				_, err = fmt.Scanf("%s", &arn)
				if err != nil {
					return c, err
				}
				c.cache.AWSRoleARN = strings.TrimSpace(arn)
			}
		}
	} else {
		// this tests if the existing creds are valid
		auth := aws.Auth{
			AccessKey: c.cache.AWSAccessKey,
			SecretKey: c.cache.AWSSecretKey,
			Token:     c.cache.AWSToken,
		}
		c.client = ec2.New(auth, aws.Regions[region])
		_, err := c.serversByProject(project)
		if err != nil {
			c.cache.AWSAccessKey = ""
			c.cache.AWSSecretKey = ""
			c.cache.AWSToken = ""
			c.cache.Save()
		}
	}
	oauthCfg.ClientId = c.cache.GoogSSOClientID
	oauthCfg.ClientSecret = c.cache.GoogSSOClientSecret
	assumeRoleARN = c.cache.AWSRoleARN
	if c.cache.AWSAccessKey == "" || c.cache.AWSSecretKey == "" {
		auth, err := authFromOAuth()
		if err != nil {
			fmt.Println("unable to get aws client")
			return c, err
		}
		c.cache.AWSAccessKey = auth.AccessKey
		c.cache.AWSSecretKey = auth.SecretKey
		c.cache.AWSToken = auth.Token
		c.cache.Save()
	}
	auth := aws.Auth{
		AccessKey: c.cache.AWSAccessKey,
		SecretKey: c.cache.AWSSecretKey,
		Token:     c.cache.AWSToken,
	}
	c.client = ec2.New(auth, aws.Regions[region])
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
func (c EC2CoreClient) Run(project string, channel string, region string, size string, num int, block bool, cloud_config string, image string) error {
	ami := image
	if image != "" {
		amis, _ := ec2GetAmis(getEc2AmiUrl(channel))
		ami = amis[region]
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
	readyc := make(chan string)
	for _, ip := range servers {
		go func(ip string) {
			for {
				_, err := net.DialTimeout("tcp", ip+":22", 400*time.Millisecond)
				if err == nil {
					fmt.Println(ip)
					readyc <- ip
					return
				}
			}

		}(ip)
	}
	ready := 0
	for {
		<-readyc
		ready++
		if ready == len(servers) {
			return
		}
	}

}
func (c EC2CoreClient) serversByProject(project string) ([]ec2.Instance, error) {
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

func (c EC2CoreClient) Terminate(project string) error {
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
func (c EC2CoreClient) List(project string) error {
	instances, err := c.serversByProject(project)
	if err != nil {
		return err
	}
	for _, instance := range instances {
		fmt.Println(instance.DNSName)
	}
	return nil
}
