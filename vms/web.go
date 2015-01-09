package vms

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"path"

	"github.com/c4milo/osx-builder/apperror"
	"github.com/c4milo/osx-builder/pkg/render"
)

var Handlers map[string]func(http.ResponseWriter, *http.Request) = map[string]func(http.ResponseWriter, *http.Request){
	"POST":   CreateVM,
	"GET":    GetVM,
	"DELETE": DestroyVM,
}

// Defines parameters supported by the CreateVM service
type CreateVMParams struct {
	VMConfig
	// Script to run inside the Guest OS upon first boot
	BootstrapScript string `json:"bootstrap_script"`
	// Callback URL to post results once the VM creation process finishes. It
	// must support POST requests and be ready to receive JSON in the body of
	// the request.
	CallbackURL string `json:"callback_url"`
}

// Invokes callback URL with results of the creation process
func sendResult(url string, value interface{}) {
	if url == "" {
		return
	}
	data, err := json.Marshal(value)
	if err != nil {
		log.Printf(`[ERROR] msg="%s" value=%+v code=%s error="%s" stacktrace=%s\n`,
			ErrCbURL.Message, value, ErrCbURL.Code, err.Error(), apperror.GetStacktrace())

		data, err = json.Marshal(ErrCbURL)
		if err != nil {
			return
		}
	}
	_, err = http.Post(url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		log.Printf(`[ERROR] msg="%s" value=%+v code=%s error="%s" stacktrace=%s\n`,
			ErrCbURL.Message, value, ErrCbURL.Code, err.Error(), apperror.GetStacktrace())
	}
}

// Creates a virtual machine with the given parameters
func CreateVM(w http.ResponseWriter, req *http.Request) {
	var params CreateVMParams
	body, err := ioutil.ReadAll(req.Body)

	if err != nil {
		log.Printf(`[ERROR] msg="%s" code=%s error="%s" stacktrace=%s\n`,
			ErrReadingReqBody.Message, ErrReadingReqBody.Code, err.Error(), apperror.GetStacktrace())

		render.JSON(w, render.Options{
			Status: ErrReadingReqBody.HTTPStatus,
			Data:   ErrReadingReqBody,
		})
		return
	}

	err = json.Unmarshal(body, &params)
	if err != nil {
		log.Printf(`[ERROR] msg="%s" code=%s error="%s" stacktrace=%s\n`,
			ErrParsingJSON.Message, ErrParsingJSON.Code, err.Error(), apperror.GetStacktrace())

		render.JSON(w, render.Options{
			Status: ErrParsingJSON.HTTPStatus,
			Data:   ErrParsingJSON,
		})
		return
	}

	b := make([]byte, 10)
	_, err = rand.Read(b)
	if err != nil {
		log.Printf(`[ERROR] msg="%s" code=%s error="%s" stacktrace=%s\n`,
			ErrInternal.Message, ErrInternal.Code, err.Error(), apperror.GetStacktrace())

		render.JSON(w, render.Options{
			Status: ErrInternal.HTTPStatus,
			Data:   ErrInternal,
		})
		return
	}

	id := fmt.Sprintf("%x", b)
	params.VMConfig.ID = id

	vm := NewVM(params.VMConfig)

	go func() {
		err := vm.Create()
		if err != nil {
			log.Printf(`[ERROR] msg="%s" value=%+v code=%s error="%s" stacktrace=%s\n`,
				ErrCreatingVM.Message, vm, ErrCreatingVM.Code, err.Error(), apperror.GetStacktrace())

			sendResult(params.CallbackURL, ErrCreatingVM)
			return
		}

		// One last effort to get an IP...
		if vm.IPAddress == "" {
			vm.Refresh()
		}

		sendResult(params.CallbackURL, vm)
	}()

	render.JSON(w, render.Options{
		Status: http.StatusAccepted,
		Data:   vm,
	})
}

// Defines parameters supported by the DestroyVM service
type DestroyVMParams struct {
	// Virtual machine ID
	ID string
}

// Destroys virtual machines by ID
func DestroyVM(w http.ResponseWriter, req *http.Request) {
	params := DestroyVMParams{
		ID: path.Base(req.URL.Path),
	}

	vm, err := FindVM(params.ID)
	if err != nil {
		log.Printf(`[ERROR] msg="%s" code=%s error="%s" stacktrace=%s\n`,
			ErrOpeningVM.Message, ErrOpeningVM.Code, err.Error(), apperror.GetStacktrace())

		render.JSON(w, render.Options{
			Status: ErrOpeningVM.HTTPStatus,
			Data:   ErrOpeningVM,
		})
		return
	}

	if vm == nil {
		log.Printf(`[ERROR] msg="%s" code=%s\n`,
			ErrVMNotFound.Message, ErrOpeningVM.Code)

		render.JSON(w, render.Options{
			Status: ErrVMNotFound.HTTPStatus,
			Data:   ErrVMNotFound,
		})
		return
	}

	err = vm.Destroy()
	if err != nil {
		log.Printf(`[ERROR] msg="%s" code=%s error="%s" stacktrace=%s\n`,
			ErrInternal.Message, ErrInternal.Code, err.Error(), apperror.GetStacktrace())

		render.JSON(w, render.Options{
			Status: ErrInternal.HTTPStatus,
			Data:   ErrInternal,
		})
		return
	}

	render.JSON(w, render.Options{
		Status: http.StatusNoContent,
	})
}

// Defines parameters supported by the GetVM service
type GetVMParams struct {
	ID string
}

// Returns information of a virtual machine given its ID
func GetVM(w http.ResponseWriter, req *http.Request) {
	params := GetVMParams{
		ID: path.Base(req.URL.Path),
	}

	vm, err := FindVM(params.ID)
	if err != nil {
		log.Printf(`[ERROR] msg="%s" code=%s error="%s" stacktrace=%s\n`,
			ErrOpeningVM.Message, ErrOpeningVM.Code, err.Error(), apperror.GetStacktrace())

		render.JSON(w, render.Options{
			Status: ErrOpeningVM.HTTPStatus,
			Data:   ErrOpeningVM,
		})
		return
	}

	if vm == nil {
		log.Printf(`[ERROR] msg="%s" code=%s\n`,
			ErrVMNotFound.Message, ErrOpeningVM.Code)

		render.JSON(w, render.Options{
			Status: ErrVMNotFound.HTTPStatus,
			Data:   ErrVMNotFound,
		})
		return
	}

	render.JSON(w, render.Options{
		Status: http.StatusOK,
		Data:   vm,
	})
}
