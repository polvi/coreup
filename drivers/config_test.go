package drivers_test

import (
	"encoding/json"
	"github.com/polvi/coreup/drivers"
	"io/ioutil"
	"os"
	"path"
	"testing"
)

func TestLoadEmpty(t *testing.T) {
	dir, err := ioutil.TempDir("", "coreup-test")
	config := path.Join(dir, ".coreup/cache.json")
	if err != nil {
		t.Fatal("error creating tmpdir")
	}
	defer os.RemoveAll(dir)

	c, err := drivers.LoadCredCache(config)
	if err != nil {
		t.Fatal("error loading cache", err)
	}
	c_dir := path.Base(config)
	if _, err := os.Stat(c_dir); os.IsNotExist(err) {
		t.Fatal("did not make .coreup dir", err)
	}
	if _, err := os.Stat(config); os.IsNotExist(err) {
		t.Fatal("did not make blank cache.json", err)
	}
	if c.RackspaceUser != "" {
		t.Fatal("rackspace should be nil")
	}
}

func TestSave(t *testing.T) {
	dir, err := ioutil.TempDir("", "coreup-test")
	config := path.Join(dir, ".coreup/cache.json")
	if err != nil {
		t.Fatal("error creating tmpdir")
	}
	defer os.RemoveAll(dir)

	c, err := drivers.LoadCredCache(config)
	c.RackspaceUser = "foo"
	c.Save()

	var t_c drivers.CredCache
	conf, err := ioutil.ReadFile(config)
	if err != nil {
		t.Fatal("config should have been written", err)
	}
	err = json.Unmarshal(conf, &t_c)
	if err != nil {
		t.Fatal("bad config", err)
	}
	if t_c.RackspaceUser != "foo" {
		t.Fatal("save got bad output")
	}
}
