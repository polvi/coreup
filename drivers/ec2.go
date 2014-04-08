package drivers

import (
	"code.google.com/p/goauth2/oauth"
	"encoding/json"
	"fmt"
	"github.com/mitchellh/goamz/aws"
	"github.com/mitchellh/goamz/ec2"
	"github.com/mitchellh/goamz/sts"
	"github.com/skratchdot/open-golang/open"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
)

var oauthCfg = &oauth.Config{
	ClientId:     "936840006820-uqofo95h28vg5iqnb1hm94lonbbve354.apps.googleusercontent.com",
	ClientSecret: "u-FtkDddDRTNzu4gO_iSJ-hd",
	AuthURL:      "https://accounts.google.com/o/oauth2/auth",
	TokenURL:     "https://accounts.google.com/o/oauth2/token",
	RedirectURL:  "http://localhost:8016/oauth2callback",
	Scope:        "https://www.googleapis.com/auth/userinfo.email",
}

const profileInfoURL = "https://www.googleapis.com/oauth2/v1/userinfo?alt=json"

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
	aws_resp, err := client.AssumeRoleWithWebIdentity(3600, "", "", "arn:aws:iam::477645798544:role/RoleForGoogle", goog.Email, token.Extra["id_token"])
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
	if err != nil {
		return aws.Auth{}, err
	}
	defer l.Close()
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
}

func EC2GetClient(project string, region string) (EC2CoreClient, error) {
	c := EC2CoreClient{}
	auth, err := authFromOAuth()
	if err != nil {
		fmt.Println("unable to get aws client")
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
	return nil
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
