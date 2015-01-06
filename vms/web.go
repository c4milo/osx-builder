package vms

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/c4milo/go-osx-builder/apperror"
	"github.com/c4milo/go-osx-builder/config"
	govix "github.com/hooklift/govix"
	"github.com/julienschmidt/httprouter"
	"github.com/satori/go.uuid"
)

func Init(router *httprouter.Router) {
	log.Infoln("Initializing vms module...")

	router.POST("/vms", CreateVM)
	router.GET("/vms", ListVMs)
	router.GET("/vms/:id", GetVM)
	router.DELETE("/vms/:id", DestroyVM)
}

type CreateVMParams struct {
	CPUs             uint              `json:"cpus"`
	Memory           string            `json:"memory"`
	NetType          govix.NetworkType `json:"network_type"`
	OSImage          Image             `json:"image"`
	BootstrapScript  string            `json:"bootstrap_script"`
	ToolsInitTimeout time.Duration     `json:"tools_init_timeout"`
	LaunchGUI        bool              `json:"launch_gui"`
	CallbackURL      string            `json:"callback_url"`
}

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
		provider:         govix.VMWARE_WORKSTATION,
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
		log.WithFields(log.Fields{
			"code":       ErrCreatingVM.Code,
			"error":      err.Error(),
			"stacktrace": apperror.GetStacktrace(),
		}).Error(ErrCreatingVM.Message)

		r.JSON(w, ErrCreatingVM.HTTPStatus, ErrCreatingVM)
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

type ListVMsParams struct {
	Status string
}

func ListVMs(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	r := config.Render

	// params := ListVMsParams{
	// 	Status: req.URL.Query().Get("status"),
	// }

	vm := VM{
		provider:  govix.VMWARE_WORKSTATION,
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
	r := config.Render

	id := ps.ByName("id")
	vm, err := FindVM(id)

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
