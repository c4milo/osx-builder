package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"time"

	govix "github.com/hooklift/govix"
	"github.com/satori/go.uuid"
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

func Log(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s %s", r.RemoteAddr, r.Method, r.URL)
		handler.ServeHTTP(w, r)
	})
}

func main() {
	http.HandleFunc("/vms", VMHandler)

	address := ":" + Port

	log.Printf("OSX Builder %s about to listen on %s", Version, address)
	err := http.ListenAndServe(address, Log(http.DefaultServeMux))
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

type BuilderError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Trace   string `json:"trace"`
}

func VMHandler(w http.ResponseWriter, req *http.Request) {
	var err error
	var vms []string
	var vminfo *VM

	switch req.Method {
	case "POST":
		var params LaunchVMParams
		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			//TODO(c4milo): Wrap error in BuilderError
			r.JSON(w, http.StatusBadRequest, err.Error())
			return
		}

		err = json.Unmarshal(body, params)
		if err != nil {
			//TODO(c4milo): Wrap error in BuilderError
			r.JSON(w, http.StatusUnsupportedMediaType, err.Error())
			return
		}

		vminfo, err = LaunchVM(params)
	case "DELETE":
		params := DestroyVMParams{
			ID: path.Base(req.URL.Path),
		}
		err = DestroyVM(params)
	case "GET":
		params := ListVMsParams{
			Status: req.URL.Query().Get("status"),
		}
		vms, err = ListVMs(params)
	default:
		r.JSON(w, http.StatusMethodNotAllowed, nil)
		return
	}

	if err != nil {
		switch err {
		default:
			r.JSON(w, http.StatusInternalServerError, err.Error())
		}
	}

	if vminfo != nil {
		r.JSON(w, http.StatusCreated, vminfo)
		return
	}

	if vms != nil {
		r.JSON(w, http.StatusOK, vms)
		return
	}

	r.JSON(w, http.StatusNotFound, nil)
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

func LaunchVM(params LaunchVMParams) (*VM, error) {
	name := uuid.NewV4()

	vm := &VM{
		Provider:         string(govix.VMWARE_WORKSTATION),
		VerifySSL:        false,
		Name:             name.String(),
		Image:            params.OSImage,
		CPUs:             params.CPUs,
		Memory:           params.Memory,
		UpgradeVHardware: false,
		ToolsInitTimeout: params.ToolsInitTimeout,
		LaunchGUI:        params.LaunchGUI,
	}

	id, err := vm.Create()
	if err != nil {
		return nil, err

	}

	if vm.IPAddress == "" {
		vm.Refresh(id)
	}

	return vm, nil
}

type DestroyVMParams struct {
	ID string
}

func DestroyVM(params DestroyVMParams) error {
	vm := VM{
		Provider:  string(govix.VMWARE_WORKSTATION),
		VerifySSL: false,
	}

	return vm.Destroy(params.ID)
}

type ListVMsParams struct {
	Status string
}

func ListVMs(params ListVMsParams) ([]string, error) {
	vm := VM{
		Provider:  string(govix.VMWARE_WORKSTATION),
		VerifySSL: false,
	}

	host, err := vm.client()
	if err != nil {
		return nil, err
	}
	defer host.Disconnect()

	return host.FindItems(govix.FIND_RUNNING_VMS)
}
