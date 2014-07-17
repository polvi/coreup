package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"time"

	"github.com/polvi/coreup/Godeps/_workspace/src/code.google.com/p/goauth2/oauth"
	"github.com/polvi/coreup/Godeps/_workspace/src/github.com/polvi/goamz/aws"
)

type ExpiringAuth struct {
	Auth   aws.Auth
	Expiry time.Time
}

type CredCache struct {
	path      string
	Rackspace struct {
		User   string
		APIKey string
	}

	GCE struct {
		SSOClientID     string
		SSOClientSecret string
		Project         string
		Token           oauth.Token
	}

	AWS struct {
		RoleARN string
		Token   ExpiringAuth
	}
}

func LoadCredCache(config string) (*CredCache, error) {
	c := CredCache{path: config}

	c_dir := path.Dir(config)
	if _, err := os.Stat(c_dir); os.IsNotExist(err) {
		err := os.MkdirAll(c_dir, 0700)
		if err != nil {
			return nil, err
		}
	}
	if _, err := os.Stat(c.path); os.IsNotExist(err) {
		err := c.Save()
		if err != nil {
			return nil, err
		}
	} else {
		c.ReadAll()
	}
	return &c, nil
}

func (c *CredCache) ReadAll() error {
	conf, err := ioutil.ReadFile(c.path)
	if err != nil {
		return err
	}
	return json.Unmarshal(conf, c)
}

func (c *CredCache) Save() error {
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		fmt.Println("error:", err)
	}
	return ioutil.WriteFile(c.path, b, 0600)
}
