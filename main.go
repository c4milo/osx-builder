package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/codegangsta/negroni"
	govix "github.com/hooklift/govix"
	"github.com/julienschmidt/httprouter"
	"github.com/meatballhat/negroni-logrus"
	"github.com/satori/go.uuid"
	"github.com/stretchr/graceful"
	"gopkg.in/unrolled/render.v1"
)

var (
	Port    string
	r       *render.Render
	Version string
)

func init() {
	Port = os.Getenv("PORT")
	if Port == "" {
		Port = "12345"
	}

	r = render.New(render.Options{
		IndentJSON: true,
	})
}

func main() {
	router := httprouter.New()

	router.POST("/vms", LaunchVM)
	router.DELETE("/vms/:id", DestroyVM)
	router.GET("/vms", ListVMs)
	router.GET("/vms/:id", GetVM)

	n := negroni.Classic()
	n.Use(negronilogrus.NewMiddleware())
	n.UseHandler(router)

	address := ":" + Port

	log.Printf("OSX Builder %s about to listen on %s", Version, address)

	graceful.Run(address, 10*time.Second, n)
}

type BuilderError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Trace   string `json:"trace"`
}

type LaunchVMParams struct {
	CPUs             uint              `json:"cpus"`
	Memory           string            `json:"memory"`
	NetType          govix.NetworkType `json:"network_type"`
	OSImage          Image             `json:"image"`
	BootstrapScript  string            `json:"bootstrap_script"`
	ToolsInitTimeout time.Duration     `json:"tools_init_timeout"`
	LaunchGUI        bool              `json:"launch_gui"`
	CallbackURL      string            `json:"callback_url"`
}

func LaunchVM(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	var params LaunchVMParams
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		//TODO(c4milo): Wrap error in BuilderError
		r.JSON(w, http.StatusBadRequest, err.Error())
		return
	}

	err = json.Unmarshal(body, &params)
	if err != nil {
		//TODO(c4milo): Wrap error in BuilderError
		r.JSON(w, http.StatusUnsupportedMediaType, err.Error())
		return
	}

	name := uuid.NewV4()

	vm := &VM{
		provider:         string(govix.VMWARE_WORKSTATION),
		verifySSL:        false,
		Name:             name.String(),
		Image:            params.OSImage,
		CPUs:             params.CPUs,
		Memory:           params.Memory,
		upgradeVHardware: false,
		ToolsInitTimeout: params.ToolsInitTimeout,
		LaunchGUI:        params.LaunchGUI,
	}

	nic := &govix.NetworkAdapter{
		ConnType: params.NetType,
	}

	vm.vNetworkAdapters = make([]*govix.NetworkAdapter, 0, 1)
	vm.vNetworkAdapters = append(vm.vNetworkAdapters, nic)

	id, err := vm.Create()
	if err != nil {
		r.JSON(w, http.StatusInternalServerError, err.Error())
		return
	}

	if vm.IPAddress == "" {
		vm.Refresh(id)
	}

	r.JSON(w, http.StatusCreated, vm)
}

type DestroyVMParams struct {
	ID string
}

func DestroyVM(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	params := DestroyVMParams{
		ID: ps.ByName("id"),
	}

	vm := VM{
		provider:  string(govix.VMWARE_WORKSTATION),
		verifySSL: false,
	}

	err := vm.Destroy(params.ID)
	if err != nil {
		r.JSON(w, http.StatusInternalServerError, err.Error())
		return
	}

	r.JSON(w, http.StatusNoContent, nil)
}

type ListVMsParams struct {
	Status string
}

func ListVMs(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	// params := ListVMsParams{
	// 	Status: req.URL.Query().Get("status"),
	// }

	vm := VM{
		provider:  string(govix.VMWARE_WORKSTATION),
		verifySSL: false,
	}

	host, err := vm.client()
	if err != nil {
		r.JSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer host.Disconnect()

	ids, err := host.FindItems(govix.FIND_RUNNING_VMS)
	if err != nil {
		r.JSON(w, http.StatusInternalServerError, err.Error())
		return
	}

	r.JSON(w, http.StatusOK, ids)
}

func GetVM(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id := ps.ByName("id")

	r.JSON(w, http.StatusOK, id)
}
