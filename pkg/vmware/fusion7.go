package vmware

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// Fusion7VM defines a VMWare Fusion7 provider.
type Fusion7VM struct {
	vmxPath   string
	vmRunPath string
}

// NewFusion7VM creates a new instance of Fusion7VM, receiving a VMX file path
// as parameter.
func NewFusion7VM(vmxPath string) *Fusion7VM {
	fusion7 := &Fusion7VM{
		vmxPath: vmxPath,
	}

	if err := fusion7.lookupVMRunPath(); err != nil {
		log.Fatalln(err)
	}

	return fusion7
}

// lookupVMRunPath finds vmrun tool in local filesystem.
func (v *Fusion7VM) lookupVMRunPath() error {
	vmrunPath := os.Getenv("VMWARE_VMRUN_PATH")

	if vmrunPath == "" {
		vmrunPath = "/Applications/VMware Fusion.app/Contents/Library/vmrun"
	}

	if _, err := os.Stat(vmrunPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("[Fusion7] VMWare vmrun program not found at path: %s", vmrunPath)
		}
	}

	v.vmRunPath = vmrunPath
	return nil
}

// verifyVMXPath verifies that the VMX file path is not empty.
func (v *Fusion7VM) verifyVMXPath() error {
	if v.vmxPath == "" {
		return errors.New("[Fusion7] Empty VMX file path. Nothing to operate on.")
	}
	return nil
}

// CloneFrom clones the source VMX file into the VMX file path of the function receiver.
func (v *Fusion7VM) CloneFrom(src string, ctype CloneType) error {
	if err := v.verifyVMXPath(); err != nil {
		return err
	}

	cmd := exec.Command(v.vmRunPath, "clone", src, v.vmxPath, string(ctype))
	if _, _, err := runAndLog(cmd); err != nil {
		return err
	}

	return nil
}

// Info returns the virtual machine information from VMWare
func (v *Fusion7VM) Info() (*VMInfo, error) {
	if err := v.verifyVMXPath(); err != nil {
		return nil, err
	}

	vmx, err := readVMXFile(v.vmxPath)
	if err != nil {
		return nil, err
	}

	info := new(VMInfo)
	info.Name = vmx["displayname"]
	info.Annotation = vmx["annotation"]

	numcpus, err := strconv.ParseInt(vmx["numvcpus"], 0, 0)
	if err != nil {
		return nil, err
	}

	memsize, err := strconv.ParseInt(vmx["memsize"], 0, 0)
	if err != nil {
		return nil, err
	}

	info.CPUs = int(numcpus)
	info.MemorySize = int(memsize)
	info.NetworkType = NetworkType(vmx["ethernet0.connectiontype"])
	info.GuestOS = vmx["guestos"]

	return info, nil
}

// SetInfo stores the VM information in VMWare
func (v *Fusion7VM) SetInfo(info *VMInfo) error {
	if err := v.verifyVMXPath(); err != nil {
		return err
	}

	vmx, err := readVMXFile(v.vmxPath)
	if err != nil {
		return err
	}

	vmx["displayname"] = info.Name
	vmx["annotation"] = info.Annotation
	vmx["numvcpus"] = strconv.Itoa(info.CPUs)
	vmx["memsize"] = strconv.Itoa(info.MemorySize)

	// This is to make sure to auto answer popups windows in the GUI. This is
	// especially helpful when running in headless mode
	vmx["msg.autoanswer"] = "true"

	// The following settings does nothing in Fusion7.
	// vmx["gui.exitatpoweroff"] = "true"
	// vmx["gui.restricted"] = "true"
	// vmx["gui.exitonclihlt"] = "true"

	// Deletes all network adapters. For the simplicity's sake
	// we are going to deliberately use only one network adapter.
	for k, _ := range vmx {
		if strings.HasPrefix(k, "ethernet") {
			delete(vmx, k)
		}
	}

	vmx["ethernet0.present"] = "true"
	vmx["ethernet0.startconnected"] = "true"
	vmx["ethernet0.virtualdev"] = "e1000"
	vmx["ethernet0.connectiontype"] = string(info.NetworkType)

	if err := writeVMXFile(v.vmxPath, vmx); err != nil {
		return err
	}

	return nil
}

// Start launches a virtual machine.
//
// Known issues:
// When starting a VM in headless mode, the VM doesn't seem to boot in VMWare Fusion 7
// It does boot, though, if we start it with headless mode disabled. Go figure why...
// The problem with starting with headless mode disabled is that it won't be possible to delete
// the VM using normal means and to overcome this we are deleting the VM from
// the filesystem by force.
func (v *Fusion7VM) Start(headless bool) error {
	if err := v.verifyVMXPath(); err != nil {
		return err
	}

	guiParam := "gui"
	if headless {
		guiParam = "nogui"
	}

	cmd := exec.Command(v.vmRunPath, "start", v.vmxPath, guiParam)
	if _, _, err := runAndLog(cmd); err != nil {
		return err
	}

	return nil
}

// Stop stops a virtual machine.
func (v *Fusion7VM) Stop() error {
	if err := v.verifyVMXPath(); err != nil {
		return err
	}

	cmd := exec.Command(v.vmRunPath, "stop", v.vmxPath)
	if _, _, err := runAndLog(cmd); err != nil {
		return err
	}

	return nil
}

// Delete removes a virtual machine using normal vmrun means.
func (v *Fusion7VM) Delete() error {
	if err := v.verifyVMXPath(); err != nil {
		return err
	}

	cmd := exec.Command(v.vmRunPath, "deleteVM", v.vmxPath)
	if _, _, err := runAndLog(cmd); err != nil {
		return err
	}

	return nil
}

// IsRunning returns whether or not a virtual machine is running.
func (v *Fusion7VM) IsRunning() (bool, error) {
	if err := v.verifyVMXPath(); err != nil {
		return false, err
	}

	cmd := exec.Command(v.vmRunPath, "list")
	stdout, _, err := runAndLog(cmd)
	if err != nil {
		return false, err
	}

	for _, line := range strings.Split(stdout, "\n") {
		if line == v.vmxPath {
			return true, nil
		}
	}

	return false, nil
}

// HasToolsInstalled returns whether or not VMWare Tools is running in the VM.
func (v *Fusion7VM) HasToolsInstalled() (bool, error) {
	if err := v.verifyVMXPath(); err != nil {
		return false, err
	}

	cmd := exec.Command(v.vmRunPath, "checkToolsState", v.vmxPath)
	stdout, _, err := runAndLog(cmd)
	if err != nil {
		return false, err
	}

	for _, line := range strings.Split(stdout, "\n") {
		if line == "installed" {
			return true, nil
		}
	}
	return false, nil
}

// IPAddress queries VMWare Tools to get the virtual machine IP address.
func (v *Fusion7VM) IPAddress() (string, error) {
	if err := v.verifyVMXPath(); err != nil {
		return "", err
	}

	cmd := exec.Command(v.vmRunPath, "getGuestIPAddress", v.vmxPath)
	stdout, _, err := runAndLog(cmd)
	if err != nil {
		return "", err
	}

	addresses := strings.Split(stdout, "\n")

	if len(addresses) > 0 {
		return addresses[0], nil
	}

	return "", nil
}

// Exists returns whether or not the VMX file for this VM exists.
func (v *Fusion7VM) Exists() (bool, error) {
	if err := v.verifyVMXPath(); err != nil {
		return false, err
	}

	if _, err := os.Stat(v.vmxPath); os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}

	return true, nil
}
