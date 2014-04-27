package drivers

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"time"
)

type CredCache struct {
	path                  string
	RackspaceUser         string
	RackspaceAPIKey       string
	GoogSSOClientID       string
	GoogSSOClientSecret   string
	AWSRoleARN            string
	AWSAccessKey          string
	AWSSecretKey          string
	AWSToken              string
	GoogAccessToken       string
	GoogIdToken           string
	GoogAccessTokenExpiry time.Time
	GoogProject           string
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
