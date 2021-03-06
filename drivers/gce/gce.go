package gce

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/polvi/coreup/Godeps/_workspace/src/code.google.com/p/goauth2/oauth"
	compute "github.com/polvi/coreup/Godeps/_workspace/src/code.google.com/p/google-api-go-client/compute/v1"
	"github.com/polvi/coreup/Godeps/_workspace/src/github.com/skratchdot/open-golang/open"
	"github.com/polvi/coreup/config"
)

const defaultRegion = "us-central1-a"
const workers = 25
const max_qps = 20

type Client struct {
	service    *compute.Service
	cache      *config.CredCache
	region     string
	project_id string
}

func GetClient(project string, region string, cache_path string) (*Client, error) {
	cache, err := config.LoadCredCache(cache_path)
	if err != nil {
		return nil, err
	}
	if region == "" {
		region = defaultRegion
	}
	if cache.GCE.Project == "" {
		var project string
		fmt.Printf("google project id: ")
		_, err = fmt.Scanf("%s", &project)
		if err != nil {
			return nil, err
		}
		cache.GCE.Project = strings.TrimSpace(project)
		cache.Save()
	}
	if cache.GCE.SSOClientID == "" || cache.GCE.SSOClientSecret == "" {
		var client_id string
		var client_secret string
		fmt.Printf("google client id: ")
		_, err = fmt.Scanf("%s", &client_id)
		if err != nil {
			return nil, err
		}
		cache.GCE.SSOClientID = strings.TrimSpace(client_id)
		fmt.Printf("google client secret: ")
		_, err = fmt.Scanf("%s", &client_secret)
		if err != nil {
			return nil, err
		}
		cache.GCE.SSOClientSecret = strings.TrimSpace(client_secret)
		if err != nil {
			return nil, err
		}
		cache.Save()
	}
	cfg := &oauth.Config{
		ClientId:     cache.GCE.SSOClientID,
		ClientSecret: cache.GCE.SSOClientSecret,
		Scope:        "https://www.googleapis.com/auth/userinfo.profile https://www.googleapis.com/auth/compute https://www.googleapis.com/auth/devstorage.read_write https://www.googleapis.com/auth/userinfo.email",
		RedirectURL:  "http://localhost:8016/oauth2callback",
		AuthURL:      "https://accounts.google.com/o/oauth2/auth",
		TokenURL:     "https://accounts.google.com/o/oauth2/token",
	}
	if cache.GCE.Token.Expiry.Before(time.Now()) {
		token, err := authRefreshToken(cfg)
		if err != nil {
			return nil, err
		}
		cache.GCE.Token = *token
		cache.Save()

	}
	token := &cache.GCE.Token
	transport := &oauth.Transport{
		Config:    cfg,
		Token:     token,
		Transport: http.DefaultTransport,
	}
	svc, err := compute.New(transport.Client())
	if err != nil {
		return nil, err
	}
	return &Client{
		service:    svc,
		cache:      cache,
		region:     region,
		project_id: cache.GCE.Project,
	}, nil
}

func authRefreshToken(c *oauth.Config) (*oauth.Token, error) {
	l, err := net.Listen("tcp", "localhost:8016")
	defer l.Close()
	if err != nil {
		return nil, err
	}
	open.Run(c.AuthCodeURL(""))
	var token *oauth.Token
	done := make(chan struct{})
	f := func(w http.ResponseWriter, r *http.Request) {
		code := r.FormValue("code")
		transport := &oauth.Transport{Config: c}
		token, err = transport.Exchange(code)
		done <- struct{}{}
	}
	go http.Serve(l, http.HandlerFunc(f))
	<-done
	return token, err
}

func (c Client) waitForOp(op *compute.Operation, zone string) error {
	op, err := c.service.ZoneOperations.Get(c.project_id, zone, op.Name).Do()
	for op.Status != "DONE" {
		time.Sleep(5 * time.Second)
		op, err = c.service.ZoneOperations.Get(c.project_id, c.region, op.Name).Do()
		if err != nil {
			return err
		}
		if op.Status != "PENDING" && op.Status != "RUNNING" && op.Status != "DONE" {
			return errors.New(fmt.Sprintf("Bad operation: %s", op))
		}
	}
	return err
}

func getLatestImg(url string) (string, error) {
	resp, err := http.Get(url)
	defer resp.Body.Close()
	if err != nil {
		return "", err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	res := strings.TrimSpace(string(body))
	return fmt.Sprintf("https://www.googleapis.com/compute/v1/%s", res), nil
}
func getSrcImg(channel string) (string, error) {
	url := fmt.Sprintf("http://storage.core-os.net/coreos/amd64-usr/%s/coreos_production_gce.txt", channel)
	img, err := getLatestImg(url)
	if err != nil {
		return "", err
	}
	return img, nil
}

func (c Client) insertWorker(id int, queue chan *compute.Instance) {
	for {
		inst, ok := <-queue
		if !ok {
			break
		}
		_, err := c.service.Instances.Insert(c.project_id, c.region, inst).Do()
		if err != nil {
			fmt.Println(err)
		}
		fmt.Printf(".")
	}
}
func (c Client) Run(project string, channel string, size string, num int, block bool, cloud_config string, image string) error {
	prefix := "https://www.googleapis.com/compute/v1/projects/" + c.project_id
	t := time.Now().Unix()
	if image == "" {
		img_src, err := getSrcImg(channel)
		if err != nil {
			return err
		}
		image = img_src
	}
	var wg sync.WaitGroup
	queue := make(chan *compute.Instance, 20)
	tick := time.Tick(time.Second)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.insertWorker(i, queue)
		}()
	}
	for i := 0; i < num; i++ {
		name := fmt.Sprintf("%s-%d-%d", project, t, i)
		instance := &compute.Instance{
			Name:        name,
			Description: project,
			MachineType: prefix + "/zones/us-central1-a/machineTypes/f1-micro",
			Disks: []*compute.AttachedDisk{
				{
					AutoDelete: true,
					Boot:       true,
					Type:       "PERSISTENT",
					Mode:       "READ_WRITE",
					InitializeParams: &compute.AttachedDiskInitializeParams{
						SourceImage: image,
					},
				},
			},
			NetworkInterfaces: []*compute.NetworkInterface{
				{
					/*
						AccessConfigs: []*compute.AccessConfig{
							&compute.AccessConfig{Type: "ONE_TO_ONE_NAT"},
						},
					*/
					Network: prefix + "/global/networks/default",
				},
			},
			Metadata: &compute.Metadata{
				Items: []*compute.MetadataItems{
					{
						Key:   "user-data",
						Value: cloud_config,
					},
				},
			},
			Tags: &compute.Tags{
				Items: []string{
					project,
				},
			},
		}
		queue <- instance
		if (i%20) == 0 && i > 0 {
			<-tick
		}
	}
	close(queue)
	wg.Wait()
	return nil
}

func (c Client) deleteWorker(id int, queue chan *compute.Instance) {
	for {
		inst, ok := <-queue
		if !ok {
			break
		}
		if inst.Status == "RUNNING" {
			fmt.Printf(".")
			_, err := c.service.Instances.Delete(c.project_id, c.region, inst.Name).Do()
			if err != nil {
				fmt.Println(err)
			}
		}
	}
}

func (c Client) Terminate(project string) error {
	var wg sync.WaitGroup
	queue := make(chan *compute.Instance)
	tick := time.Tick(time.Second)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.deleteWorker(i, queue)
		}()
	}
	filter := fmt.Sprintf("name eq %s.*", project)
	req := c.service.Instances.List(c.project_id, c.region).Filter(filter)
	instances, err := req.Do()
	if err != nil {
		return err
	}
	for i, instance := range instances.Items {
		queue <- instance
		if (i%20) == 0 && i > 0 {
			<-tick
		}
	}
	for {
		if instances.NextPageToken == "" {
			break
		}
		req.PageToken(instances.NextPageToken)
		instances, err = req.Do()
		if err != nil {
			return err
		}
		for i, instance := range instances.Items {
			queue <- instance
			if (i % 20) == 0 {
				<-tick
			}
		}
	}
	close(queue)
	wg.Wait()
	return nil
}

func printInstances(instances *compute.InstanceList) {
	for _, instance := range instances.Items {
		fmt.Println(instance)
	}
}
func (c Client) List(project string) error {
	// TODO would be more ideal to filter on tags, but I could not find that
	filter := fmt.Sprintf("name eq %s.*", project)
	req := c.service.Instances.List(c.project_id, c.region).Filter(filter)
	instances, err := req.Do()
	if err != nil {
		return err
	}
	printInstances(instances)
	for {
		if instances.NextPageToken == "" {
			break
		}
		req.PageToken(instances.NextPageToken)
		instances, err = req.Do()
		if err != nil {
			return err
		}
		printInstances(instances)
	}
	return nil
}
