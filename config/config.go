package config

import (
	"os"
	"os/user"
	"path/filepath"

	"gopkg.in/unrolled/render.v1"
)

var (
	// Render instance to easily serialize and return data to HTTP clients
	Render *render.Render
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

	Render = render.New(render.Options{
		IndentJSON: true,
	})

	usr, err := user.Current()
	if err != nil {
		panic(err)
	}

	basePath := filepath.Join(usr.HomeDir, ".go-osx-builder", "vix")
	VMSPath = filepath.Join(basePath, "vms")
	GoldImgsPath = filepath.Join(basePath, "gold")
	ImagesPath = filepath.Join(basePath, "images")
}
