package config

import (
	"os"
	"os/user"
	"path/filepath"

	"gopkg.in/unrolled/render.v1"
)

var (
	Render       *render.Render
	Port         string
	VMSPath      string
	GoldImgsPath string
	ImagesPath   string
)

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
