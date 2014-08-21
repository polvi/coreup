package do

import (
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/polvi/coreup/Godeps/_workspace/src/code.google.com/p/goauth2/oauth"
	do "github.com/polvi/coreup/Godeps/_workspace/src/github.com/dynport/gocloud/digitalocean/v2/digitalocean"
	"github.com/polvi/coreup/Godeps/_workspace/src/github.com/skratchdot/open-golang/open"
	"github.com/polvi/coreup/config"
)

const defaultRegion = "us-central1-a"

type Client struct {
	service *do.Client
	cache   *config.CredCache
	region  string
}

func GetClient(project string, region string, cache_path string) (*Client, error) {
	cache, err := config.LoadCredCache(cache_path)
	if err != nil {
		return nil, err
	}
	if region == "" {
		region = defaultRegion
	}
	cfg := &oauth.Config{
		ClientId:     "69d74afe2eb5cfde808a333e448cdd2a0bd60672ab483850ac38fe68b383e1db",
		ClientSecret: "cf430afc2cff561a97883309700dc7966fec0ca5520e2a0fe0e2d8b382f6538a",
		Scope:        "read write",
		RedirectURL:  "http://localhost:8016/oauth2callback",
		AuthURL:      "https://cloud.digitalocean.com/v1/oauth/authorize",
		TokenURL:     "https://cloud.digitalocean.com/v1/oauth/token",
	}
	if cache.DO.Token.Expiry.Before(time.Now()) {
		token, err := authRefreshToken(cfg)
		if err != nil {
			return nil, err
		}
		cache.DO.Token = *token
		cache.Save()
	}
	token := &cache.DO.Token
	transport := &oauth.Transport{
		Config:    cfg,
		Token:     token,
		Transport: http.DefaultTransport,
	}
	svc := &do.Client{
		Client: transport.Client(),
	}
	if err != nil {
		return nil, err
	}
	return &Client{
		service: svc,
		cache:   cache,
		region:  region,
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

func (c Client) Run(project string, channel string, size string, num int, block bool, cloud_config string, image string) error {

	create := do.CreateDroplet{
		Name:     project,
		Region:   "nyc3",
		Size:     "512mb",
		Image:    "5648377",
		UserData: cloud_config,
	}
	_, err := create.Execute(c.service)
	if err != nil {
		return err
	}
	return nil
}

func (c Client) Terminate(project string) error {
	ds, err := c.service.Droplets()
	if err != nil {
		return err
	}
	for _, d := range ds.Droplets {
		if d.Name == project {
			err := c.service.DropletDelete(string(d.Id))
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (c Client) List(project string) error {
	ds, err := c.service.Droplets()
	if err != nil {
		return err
	}
	for _, d := range ds.Droplets {
		for _, n := range d.Networks.V4 {
			fmt.Println(n.IpAddress)
		}
	}
	return nil
}
