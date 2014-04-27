package drivers

import (
	"errors"
	"fmt"
	"github.com/skratchdot/open-golang/open"
	"net"
	"net/http"
	"strings"
	"time"

	"code.google.com/p/goauth2/oauth"
	compute "code.google.com/p/google-api-go-client/compute/v1"
)

const defaultGCERegion = "us-central1-a"

type GCECoreClient struct {
	service    *compute.Service
	cache      *CredCache
	region     string
	project_id string
}

func GCEGetClient(project string, region string, cache_path string) (*GCECoreClient, error) {
	cache, err := LoadCredCache(cache_path)
	if err != nil {
		return nil, err
	}
	if region == "" {
		region = defaultGCERegion
	}
	if cache.GoogProject == "" {
		var project string
		fmt.Printf("google project id: ")
		_, err = fmt.Scanf("%s", &project)
		if err != nil {
			return nil, err
		}
		cache.GoogProject = strings.TrimSpace(project)
		cache.Save()
	}
	if cache.GoogSSOClientID == "" || cache.GoogSSOClientSecret == "" {
		var client_id string
		var client_secret string
		fmt.Printf("google client id: ")
		_, err = fmt.Scanf("%s", &client_id)
		if err != nil {
			return nil, err
		}
		cache.GoogSSOClientID = strings.TrimSpace(client_id)
		fmt.Printf("google client secret: ")
		_, err = fmt.Scanf("%s", &client_secret)
		if err != nil {
			return nil, err
		}
		cache.GoogSSOClientSecret = strings.TrimSpace(client_secret)
		if err != nil {
			return nil, err
		}
		cache.Save()
	}
	cfg := &oauth.Config{
		ClientId:     cache.GoogSSOClientID,
		ClientSecret: cache.GoogSSOClientSecret,
		Scope:        "https://www.googleapis.com/auth/userinfo.profile https://www.googleapis.com/auth/compute https://www.googleapis.com/auth/devstorage.read_write",
		RedirectURL:  "http://localhost:8016/oauth2callback",
		AuthURL:      "https://accounts.google.com/o/oauth2/auth",
		TokenURL:     "https://accounts.google.com/o/oauth2/token",
	}
	if cache.GoogAccessToken == "" {
		token, err := authRefreshToken(cfg)
		if err != nil {
			return nil, err
		}
		cache.GoogAccessToken = token.AccessToken
		cache.Save()

	}
	token := cache.GoogAccessToken
	transport := &oauth.Transport{
		Config: cfg,
		Token: &oauth.Token{
			AccessToken: token,
		},
		Transport: http.DefaultTransport,
	}
	svc, err := compute.New(transport.Client())
	if err != nil {
		return nil, err
	}
	return &GCECoreClient{
		service:    svc,
		cache:      cache,
		region:     region,
		project_id: cache.GoogProject,
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

func (c GCECoreClient) waitForOp(op *compute.Operation, zone string) error {
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

func (c GCECoreClient) Run(project string, channel string, region string, size string, num int, block bool, cloud_config string, image string) error {
	prefix := "https://www.googleapis.com/compute/v1/projects/" + c.project_id
	time := time.Now().Unix()
	for i := 0; i < num; i++ {
		name := fmt.Sprintf("%s-%d-%d", project, time, i)
		instance := &compute.Instance{
			Name:        name,
			Description: project,
			MachineType: prefix + "/zones/us-central1-a/machineTypes/n1-standard-1",
			Disks: []*compute.AttachedDisk{
				{
					AutoDelete: true,
					Boot:       true,
					Type:       "PERSISTENT",
					Mode:       "READ_WRITE",
					InitializeParams: &compute.AttachedDiskInitializeParams{
						SourceImage: "https://www.googleapis.com/compute/v1/projects/coreos-coreup/global/images/coreos-v298-0-0",
					},
				},
			},
			NetworkInterfaces: []*compute.NetworkInterface{
				{
					AccessConfigs: []*compute.AccessConfig{
						&compute.AccessConfig{Type: "ONE_TO_ONE_NAT"},
					},
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
		_, err := c.service.Instances.Insert(c.project_id, c.region, instance).Do()
		if err != nil {
			return err
		}
	}
	return nil
}

func (c GCECoreClient) Terminate(project string) error {
	filter := fmt.Sprintf("name eq %s.*", project)
	instances, err := c.service.Instances.List(c.project_id, c.region).Filter(filter).Do()
	if err != nil {
		return err
	}
	for _, instance := range instances.Items {
		_, err := c.service.Instances.Delete(c.project_id, c.region, instance.Name).Do()
		if err != nil {
			return err
		}
	}
	return nil
}

func (c GCECoreClient) List(project string) error {
	// TODO would be more ideal to filter on tags, but I could not find that
	filter := fmt.Sprintf("name eq %s.*", project)
	instances, err := c.service.Instances.List(c.project_id, c.region).Filter(filter).Do()
	if err != nil {
		return err
	}
	for _, instance := range instances.Items {
		fmt.Println(instance.NetworkInterfaces[0].AccessConfigs[0].NatIP)
	}
	return nil
}
