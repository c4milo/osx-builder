package config

import (
	"os"
	"os/user"
	"path/filepath"
)

var (
	// Port for HTTP service to bind to
	Port string
	// Where all the virtual machines are going to be created
	VMSPath string
	// Where all the gold images are going to be cached
	GoldImgsPath string
	// Where all the raw images are downloaded to
	ImagesPath string
)

// Initializes service's configuration
func init() {
	Port = os.Getenv("PORT")
	if Port == "" {
		Port = "12345"
	}

	usr, err := user.Current()
	if err != nil {
		panic(err)
	}

	basePath := filepath.Join(usr.HomeDir, ".osx-builder")
	VMSPath = filepath.Join(basePath, "vms")
	GoldImgsPath = filepath.Join(basePath, "gold")
	ImagesPath = filepath.Join(basePath, "images")
}
