package vms

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"

	log "github.com/Sirupsen/logrus"
	govix "github.com/hooklift/govix"

	"github.com/c4milo/go-osx-builder/apperror"
	"github.com/c4milo/go-osx-builder/config"
	"github.com/julienschmidt/httprouter"
	"github.com/satori/go.uuid"
)

// Initializes module
func Init(router *httprouter.Router) {
	log.Infoln("Initializing vms module...")

	router.POST("/vms", CreateVM)
	//router.GET("/vms", ListVMs)
	router.GET("/vms/:id", GetVM)
	router.DELETE("/vms/:id", DestroyVM)
}

// Defines parameters supported by the CreateVM service
type CreateVMParams struct {
	// Number of virtual cpus to assign to the VM
	CPUs uint `json:"cpus"`
	// Memory for the virtual machine in IEC units. Ex: 1024mib, 1gib, 5120kib,
	Memory string `json:"memory"`
	// Network type, either "bridged", "nat" or "hostonly"
	NetType govix.NetworkType `json:"network_type"`
	// Guest OS image that is going to be used as Gold image for creating new VMs
	OSImage Image `json:"image"`
	// Script to run inside the Guest OS upon first boot
	BootstrapScript string `json:"bootstrap_script"`
	// Timeout value for waiting for VMWare Tools to initialize
	ToolsInitTimeout time.Duration `json:"tools_init_timeout"`
	// Whether or not to launch the user interface when creating the VM
	LaunchGUI bool `json:"launch_gui"`
	// Callback URL to post results once the VM creation process finishes. It
	// must support POST requests and be ready to receive JSON in the body of
	// the request.
	CallbackURL string `json:"callback_url"`
}

// Invokes callback URL with the results of the creation process
func sendResult(url string, obj interface{}) {
	if url == "" {
		return
	}
	data, err := json.Marshal(obj)
	if err != nil {
		log.WithFields(log.Fields{
			"vm":         obj,
			"code":       ErrCreatingVM.Code,
			"error":      err.Error(),
			"stacktrace": apperror.GetStacktrace(),
		}).Error(ErrCbURL.Message)

		data, err = json.Marshal(ErrCbURL)
		if err != nil {
			return
		}
	}
	_, err = http.Post(url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		log.WithFields(log.Fields{
			"vm":         obj,
			"code":       ErrCbURL.Code,
			"error":      err.Error(),
			"stacktrace": apperror.GetStacktrace(),
		}).Error(ErrCbURL.Message)
	}
}

// Creates a virtual machine with the given parameters
func CreateVM(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	r := config.Render

	var params CreateVMParams
	body, err := ioutil.ReadAll(req.Body)

	if err != nil {
		log.WithFields(log.Fields{
			"code":       ErrReadingReqBody.Code,
			"error":      err.Error(),
			"stacktrace": apperror.GetStacktrace(),
		}).Error(ErrReadingReqBody.Message)

		r.JSON(w, ErrReadingReqBody.HTTPStatus, ErrReadingReqBody)
		return
	}

	err = json.Unmarshal(body, &params)
	if err != nil {
		log.WithFields(log.Fields{
			"code":       ErrParsingJSON.Code,
			"error":      err.Error(),
			"stacktrace": apperror.GetStacktrace(),
		}).Error(ErrParsingJSON.Message)

		r.JSON(w, ErrParsingJSON.HTTPStatus, ErrParsingJSON)
		return
	}

	name := uuid.NewV4()

	vm := &VM{
		Provider:         govix.VMWARE_WORKSTATION,
		VerifySSL:        false,
		Name:             name.String(),
		Image:            params.OSImage,
		CPUs:             params.CPUs,
		Memory:           params.Memory,
		UpgradeVHardware: false,
		ToolsInitTimeout: params.ToolsInitTimeout,
		LaunchGUI:        params.LaunchGUI,
	}

	nic := &govix.NetworkAdapter{
		ConnType: params.NetType,
	}

	vm.VNetworkAdapters = make([]*govix.NetworkAdapter, 0, 1)
	vm.VNetworkAdapters = append(vm.VNetworkAdapters, nic)

	go func() {
		id, err := vm.Create()
		if err != nil {
			log.WithFields(log.Fields{
				"vm":         vm,
				"code":       ErrCreatingVM.Code,
				"error":      err.Error(),
				"stacktrace": apperror.GetStacktrace(),
			}).Error(ErrCreatingVM.Message)

			sendResult(params.CallbackURL, ErrCreatingVM)
			return
		}

		if vm.IPAddress == "" {
			vm.Refresh(id)
		}

		sendResult(params.CallbackURL, vm)
	}()

	r.JSON(w, http.StatusAccepted, vm)
}

// Defines parameters supported by the DestroyVM service
type DestroyVMParams struct {
	// Virtual machine ID to destroy
	ID string
}

// Destroys virtual machines by ID
func DestroyVM(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	r := config.Render

	params := DestroyVMParams{
		ID: ps.ByName("id"),
	}

	vm, err := FindVM(params.ID)
	if err != nil {
		log.WithFields(log.Fields{
			"code":       ErrOpeningVM.Code,
			"error":      err.Error(),
			"stacktrace": apperror.GetStacktrace(),
		}).Error(ErrOpeningVM.Message)

		r.JSON(w, ErrOpeningVM.HTTPStatus, ErrOpeningVM)
		return
	}

	if vm == nil {
		log.WithFields(log.Fields{
			"code":       ErrVMNotFound.Code,
			"error":      "",
			"stacktrace": "",
		}).Error(ErrVMNotFound.Message)

		r.JSON(w, ErrVMNotFound.HTTPStatus, ErrVMNotFound)
		return
	}

	err = vm.Destroy(vm.VMXFile)
	if err != nil {
		log.WithFields(log.Fields{
			"code":       ErrInternal.Code,
			"error":      err.Error(),
			"stacktrace": apperror.GetStacktrace(),
		}).Error(ErrInternal.Message)

		r.JSON(w, ErrInternal.HTTPStatus, ErrInternal)
		return
	}

	r.JSON(w, http.StatusNoContent, nil)
}

// Defines parameters supported by the ListVMs service
type ListVMsParams struct {
	Status string
}

// Returns the list of registered machines in VMWare Fusion/Workstation
// Due to limitations in VMware VIX, it only returns running machines.
func ListVMs(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	r := config.Render

	// params := ListVMsParams{
	// 	Status: req.URL.Query().Get("status"),
	// }

	vm := VM{
		Provider:  govix.VMWARE_WORKSTATION,
		VerifySSL: false,
	}

	host, err := vm.client()
	if err != nil {
		log.WithFields(log.Fields{
			"code":       ErrInternal.Code,
			"error":      err.Error(),
			"stacktrace": apperror.GetStacktrace(),
		}).Error(ErrInternal.Message)

		r.JSON(w, ErrInternal.HTTPStatus, ErrInternal)
		return
	}
	defer host.Disconnect()

	ids, err := host.FindItems(govix.FIND_RUNNING_VMS)
	if err != nil {
		log.WithFields(log.Fields{
			"code":       ErrInternal.Code,
			"error":      err.Error(),
			"stacktrace": apperror.GetStacktrace(),
		}).Error(ErrInternal.Message)

		r.JSON(w, ErrInternal.HTTPStatus, ErrInternal)
		return
	}

	r.JSON(w, http.StatusOK, ids)
}

// Defines parameters supported by the GetVM service
type GetVMParams struct {
	ID string
}

// Returns information of a virtual machine given its ID
func GetVM(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	r := config.Render

	params := GetVMParams{
		ID: ps.ByName("id"),
	}

	vm, err := FindVM(params.ID)

	if err != nil {
		log.WithFields(log.Fields{
			"code":       ErrOpeningVM.Code,
			"error":      err.Error(),
			"stacktrace": apperror.GetStacktrace(),
		}).Error(ErrOpeningVM.Message)

		r.JSON(w, ErrOpeningVM.HTTPStatus, ErrOpeningVM)
		return
	}

	if vm == nil {
		log.WithFields(log.Fields{
			"code":       ErrVMNotFound.Code,
			"error":      "",
			"stacktrace": "",
		}).Error(ErrVMNotFound.Message)

		r.JSON(w, ErrVMNotFound.HTTPStatus, ErrVMNotFound)
		return
	}

	r.JSON(w, http.StatusOK, vm)
}
