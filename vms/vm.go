// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vms

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"

	"github.com/c4milo/osx-builder/config"
	"github.com/c4milo/osx-builder/pkg/unzipit"
	"github.com/c4milo/osx-builder/pkg/vmware"
)

// VMConfig defines all the configurable properties of a virtual machine.
type VMConfig struct {
	// ID of the virtual machine
	ID string `json:"id"`
	// Image to use during the creation of this virtual machine
	OSImage Image `json:"image"`
	// Number of virtual cpus
	CPUs int `json:"cpus"`
	// Memory size in megabytes.
	Memory int `json:"memory"`
	// Network adapters
	Network vmware.NetworkType `json:"network_type"`
	// Whether to launch the VM with graphical environment
	Headless bool `json:"headless"`
}

// VM defines the properties of a virtual machine.
type VM struct {
	VMConfig
	// Underlined VMWare virtual machine
	vmwareVM vmware.VirtualMachine
	// VM IP address as reported by VMWare
	IPAddress string `json:"ip_address"`
	// Power status
	Status string `json:"status"`
}

// NewVM creates a new instance of VM.
func NewVM(c VMConfig) *VM {
	vmxfile := filepath.Join(config.VMSPath, c.ID, c.ID+".vmx")

	return &VM{
		VMConfig: c,
		vmwareVM: vmware.NewFusion7VM(vmxfile),
	}
}

// setDefaults assigns defalt values for some VM properties.
func (v *VM) setDefaults() {
	if v.CPUs <= 0 {
		v.CPUs = 2
	}

	if v.Memory < 512 {
		v.Memory = 512
	}
}

// unpackGoldImage fetches and decompresses the Gold OS image.
func (v *VM) unpackGoldImage() (string, error) {
	image := v.OSImage
	goldPath := filepath.Join(config.GoldImgsPath, image.Checksum)

	_, err := os.Stat(goldPath)
	finfo, _ := ioutil.ReadDir(goldPath)
	goldPathEmpty := len(finfo) == 0

	if os.IsNotExist(err) || goldPathEmpty {
		log.Println("[DEBUG] Gold virtual machine does not exist or is empty")

		imgPath := filepath.Join(config.ImagesPath, image.Checksum)
		if err = image.Download(imgPath); err != nil {
			return "", err
		}
		defer image.file.Close()

		// Makes sure file cursor is in the right position.
		_, err := image.file.Seek(0, 0)
		if err != nil {
			return "", err
		}

		log.Printf("[DEBUG] Unpacking gold virtual machine into %s\n", goldPath)
		_, err = unzipit.Unpack(image.file, goldPath)
		if err != nil {
			debug.PrintStack()
			log.Printf("[ERROR] Unpacking gold image %s\n", image.file.Name())
			return "", err
		}
	}

	return goldPath, nil
}

// Create creates and launches a virtual machine.
func (v *VM) Create() error {
	log.Printf("[DEBUG] Creating VM %s", v.ID)

	goldPath, err := v.unpackGoldImage()
	if err != nil {
		return err
	}

	pattern := filepath.Join(goldPath, "**.vmx")

	log.Printf("[DEBUG] Finding gold vmx file in %s", pattern)
	files, _ := filepath.Glob(pattern)

	if len(files) == 0 {
		return fmt.Errorf("[ERROR] Gold vmx file was not found: %s", pattern)
	}

	goldvmx := files[0]
	log.Printf("[DEBUG] Gold vmx file found at %v", goldvmx)

	vmexists, err := v.vmwareVM.Exists()
	if err != nil {
		return err
	}

	if !vmexists {
		err := v.vmwareVM.CloneFrom(goldvmx, vmware.CloneLinked)
		if err != nil {
			return err
		}
	}

	if err = v.Update(); err != nil {
		return err
	}

	return nil
}

// Updates a virtual machine.
func (v *VM) Update() error {
	v.setDefaults()

	running, err := v.vmwareVM.IsRunning()
	if err != nil {
		return err
	}

	if running {
		log.Printf("[INFO] Virtual machine seems to be running, we need to " +
			"power it off in order to make changes.")
		err = v.vmwareVM.Stop()
		if err != nil {
			return err
		}
	}

	info := &vmware.VMInfo{
		//hacky way of making sure it is a multiple of 4 megabytes
		MemorySize: (v.Memory + 3) & ^0x03,
		CPUs:       v.CPUs,
		Name:       v.ID,
	}

	imageJSON, err := json.Marshal(v.OSImage)
	if err != nil {
		return err
	}

	// Encodes JSON data as Base64 so that the VMX file is not
	// interpreted by VMWare as corrupted.
	info.Annotation = base64.StdEncoding.EncodeToString(imageJSON)

	log.Printf("[DEBUG] Adding network adapter...")
	info.NetworkType = v.Network

	err = v.vmwareVM.SetInfo(info)
	if err != nil {
		return err
	}

	log.Println("[INFO] Powering virtual machine on...")
	err = v.vmwareVM.Start(v.Headless)
	if err != nil {
		return err
	}

	return nil
}

// Destroy removes a virtual machine.
func (v *VM) Destroy() error {
	running, err := v.vmwareVM.IsRunning()
	if err != nil {
		return err
	}

	if running {
		log.Printf("[DEBUG] Stopping %s...", v.ID)
		if err = v.vmwareVM.Stop(); err != nil {
			return err
		}
		log.Printf("[DEBUG] %s stopped", v.ID)
	}

	// We are not handling errors here on purpose and due to vmrun limitations
	v.vmwareVM.Delete()

	if v.ID != "" {
		os.RemoveAll(filepath.Join(config.VMSPath, v.ID))
	}

	return nil
}

// FindVM finds a virtual machine by ID.
func FindVM(id string) (*VM, error) {
	vm := NewVM(VMConfig{
		ID: id,
	})

	exists, err := vm.vmwareVM.Exists()
	if err != nil {
		return nil, err
	}

	if !exists {
		return nil, nil
	}

	err = vm.Refresh()
	if err != nil {
		return nil, err
	}

	return vm, nil
}

// Refresh synchronizes VM state against VMware.
func (v *VM) Refresh() error {
	log.Printf("[DEBUG] Refreshing state with VMWare...")
	info, err := v.vmwareVM.Info()
	if err != nil {
		return err
	}

	v.CPUs = info.CPUs
	v.Memory = info.MemorySize
	v.ID = info.Name

	v.IPAddress, _ = v.vmwareVM.IPAddress()

	imageJSONBase64 := info.Annotation
	imageJSON, err := base64.StdEncoding.DecodeString(imageJSONBase64)
	if err != nil {
		return err
	}

	var image Image
	err = json.Unmarshal(imageJSON, &image)
	if err != nil {
		return err
	}
	v.OSImage = image
	v.Network = info.NetworkType

	running, err := v.vmwareVM.IsRunning()
	if err != nil {
		return err
	}

	if running {
		v.Status = "running"
	} else {
		v.Status = "stopped"
	}

	log.Printf("[DEBUG] Finished refreshing state from VMWare")
	return nil
}
