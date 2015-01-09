package vms

import (
	"encoding/base64"
	"encoding/json"
	"errors"
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
	GUI bool `json:"gui"`
}

// Virtual machine configuration
type VM struct {
	VMConfig
	// Underlined virtual machine manager
	manager vmware.VMManager
	// Internal reference to the VM's vmx file
	vmxfile string `json:"-"`
	// VM IP address as reported by VMWare
	IPAddress string `json:"ip_address"`
	// Power status
	Status string `json:"status"`
}

func NewVM(vmcfg VMConfig) *VM {
	vmxfile := filepath.Join(config.VMSPath, vmcfg.ID, vmcfg.ID+".vmx")

	return &VM{
		VMConfig: vmcfg,
		manager:  new(vmware.Fusion7),
		vmxfile:  vmxfile,
	}
}

// Sets default values for VM attributes
func (v *VM) setDefaults() {
	if v.CPUs <= 0 {
		v.CPUs = 2
	}

	if v.Memory < 512 {
		v.Memory = 512
	}
}

// Downloads OS image, creates and launches a virtual machine.
func (v *VM) Create() error {
	log.Printf("[DEBUG] Creating VM %s", v.ID)

	image := v.OSImage
	goldPath := filepath.Join(config.GoldImgsPath, image.Checksum)

	_, err := os.Stat(goldPath)
	finfo, _ := ioutil.ReadDir(goldPath)
	goldPathEmpty := len(finfo) == 0

	if os.IsNotExist(err) || goldPathEmpty {
		log.Println("[DEBUG] Gold virtual machine does not exist or is empty")

		imgPath := filepath.Join(config.ImagesPath, image.Checksum)
		if err = image.Download(imgPath); err != nil {
			return err
		}
		defer image.file.Close()

		// Makes sure file cursor is in the right position.
		_, err := image.file.Seek(0, 0)
		if err != nil {
			return err
		}

		log.Printf("[DEBUG] Unpacking gold virtual machine into %s\n", goldPath)
		_, err = unzipit.Unpack(image.file, goldPath)
		if err != nil {
			debug.PrintStack()
			log.Printf("[ERROR] Unpacking gold image %s\n", image.file.Name())
			return err
		}
	}

	pattern := filepath.Join(goldPath, "**.vmx")

	log.Printf("[DEBUG] Finding gold vmx file in %s", pattern)
	files, _ := filepath.Glob(pattern)

	if len(files) == 0 {
		return fmt.Errorf("[ERROR] Gold vmx file was not found: %s", pattern)
	}

	goldvmx := files[0]
	log.Printf("[DEBUG] Gold vmx file found at %v", goldvmx)

	vmFolder := filepath.Join(config.VMSPath, v.ID)
	clonevmx := filepath.Join(config.VMSPath, v.ID, v.ID+".vmx")

	if _, err = os.Stat(clonevmx); os.IsNotExist(err) {
		log.Printf("[INFO] Virtual machine clone not found: %s, err: %+v", clonevmx, err)
		// If there is not a VMX file, make sure nothing else is in there either.
		os.RemoveAll(vmFolder)

		log.Printf("[INFO] Cloning gold vmx into %s...", clonevmx)
		err := v.manager.Clone(goldvmx, clonevmx, vmware.CloneLinked)
		if err != nil {
			return err
		}
	} else {
		log.Printf("[INFO] Clone %s already exist, moving on.", clonevmx)
	}

	v.vmxfile = clonevmx

	if err = v.Update(); err != nil {
		return err
	}

	return nil
}

// Updates virtual machine
func (v *VM) Update() error {
	if v.vmxfile == "" {
		return errors.New("Empty vmxfile. Nothing to update.")
	}
	v.setDefaults()

	running, err := v.manager.IsRunning(v.vmxfile)
	if err != nil {
		return err
	}

	if running {
		log.Printf("[INFO] Virtual machine seems to be running, we need to " +
			"power it off in order to make changes.")
		err = v.manager.Stop(v.vmxfile)
		if err != nil {
			return err
		}
	}

	info := &vmware.VMInfo{
		MemorySize: v.Memory,
		CPUs:       v.CPUs,
		Name:       v.ID,
	}

	imageJSON, err := json.Marshal(v.OSImage)
	if err != nil {
		return err
	}

	// We need to encode the JSON data in base64 so that the VMX file is not
	// interpreted by VMWare as corrupted.
	info.Annotation = base64.StdEncoding.EncodeToString(imageJSON)

	log.Printf("[DEBUG] Adding network adapter...")
	info.NetworkAdapters = []vmware.NetworkAdapter{
		vmware.NetworkAdapter{NetType: v.Network},
	}

	err = v.manager.SetInfo(info)
	if err != nil {
		return err
	}

	log.Println("[INFO] Powering virtual machine on...")
	err = v.manager.Start(v.vmxfile, v.GUI)
	if err != nil {
		return err
	}

	return nil
}

// Destroys a virtual machine resource
func (v *VM) Destroy() error {
	if v.vmxfile == "" {
		return errors.New("Empty vmxfile. Nothing to destroy.")
	}

	running, err := v.manager.IsRunning(v.vmxfile)
	if err != nil {
		return err
	}

	if running {
		log.Printf("[DEBUG] Stopping %s...", v.vmxfile)
		if err = v.manager.Stop(v.vmxfile); err != nil {
			return err
		}
	}

	log.Printf("[DEBUG] Destroying %s...", v.vmxfile)
	err = v.manager.Delete(v.vmxfile)
	if err != nil {
		return err
	}

	// Just in case
	if v.ID != "" {
		os.RemoveAll(filepath.Join(config.VMSPath, v.ID))
	}

	log.Printf("[DEBUG] VM %s Destroyed.", v.vmxfile)
	return nil
}

// Finds a virtual machine by ID
func FindVM(id string) (*VM, error) {
	vmxfile := filepath.Join(config.VMSPath, id, id+".vmx")

	_, err := os.Stat(vmxfile)
	if os.IsNotExist(err) {
		return nil, nil
	}

	vm := NewVM(VMConfig{
		ID: id,
	})

	log.Printf("Getting VM information from %s...", vm.vmxfile)
	err = vm.Refresh()
	if err != nil {
		return nil, err
	}

	return vm, nil
}

// Refreshes state with VMware
func (v *VM) Refresh() error {
	if v.vmxfile == "" {
		return errors.New("Empty vmxfile. Nothing to refresh.")
	}

	log.Printf("[DEBUG] Refreshing state with VMWare %s...", v.vmxfile)
	info, err := v.manager.Info(v.vmxfile)
	if err != nil {
		return err
	}

	v.CPUs = info.CPUs
	v.Memory = info.MemorySize
	v.ID = info.Name

	v.IPAddress, err = v.manager.IPAddress(v.vmxfile)
	if err != nil {
		return err
	}

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

	if len(info.NetworkAdapters) > 0 {
		v.Network = info.NetworkAdapters[0].NetType
	}

	running, err := v.manager.IsRunning(v.vmxfile)
	if err != nil {
		return err
	}

	if running {
		v.Status = "running"
	} else {
		v.Status = "stopped"
	}

	log.Printf("[DEBUG] Finished refreshing state from %s...", v.vmxfile)
	return nil
}
