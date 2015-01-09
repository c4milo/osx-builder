package vmware

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
)

type Fusion7VM struct {
	vmxPath   string
	vmRunPath string
}

func NewFusion7VM(vmxPath string) *Fusion7VM {
	fusion7 := &Fusion7VM{
		vmxPath: vmxPath,
	}

	if err := fusion7.lookupVMRunPath(); err != nil {
		log.Fatalln(err)
	}

	return fusion7
}

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

func (v *Fusion7VM) verifyVMXPath() error {
	if v.vmxPath == "" {
		return errors.New("[Fusion7] Empty VMX file path. Nothing to operate on.")
	}
	return nil
}

func (v *Fusion7VM) CloneFrom(srcfile string, ctype CloneType) error {
	if err := v.verifyVMXPath(); err != nil {
		return err
	}

	cmd := exec.Command(v.vmRunPath, "clone", srcfile, v.vmxPath, string(ctype))
	if _, _, err := runAndLog(cmd); err != nil {
		return err
	}

	return nil
}

func (v *Fusion7VM) Info() (*VMInfo, error) {
	if err := v.verifyVMXPath(); err != nil {
		return nil, err
	}

	// info := new(VMInfo)
	// info.Annotation
	// info.CPUs
	// info.MemorySize
	// info.Name
	// info.NetworkAdapters

	return nil, nil
}

func (v *Fusion7VM) SetInfo(info *VMInfo) error {
	if err := v.verifyVMXPath(); err != nil {
		return err
	}
	return nil
}

func (v *Fusion7VM) Start(headless bool) error {
	if err := v.verifyVMXPath(); err != nil {
		return err
	}

	guiParam := "nogui"
	if headless {
		guiParam = "gui"
	}

	cmd := exec.Command(v.vmRunPath, "start", v.vmxPath, guiParam)
	if _, _, err := runAndLog(cmd); err != nil {
		return err
	}

	return nil
}

func (v *Fusion7VM) Stop() error {
	if err := v.verifyVMXPath(); err != nil {
		return err
	}
	return nil

	cmd := exec.Command(v.vmRunPath, "stop", v.vmxPath)
	if _, _, err := runAndLog(cmd); err != nil {
		return err
	}

	return nil
}

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

func (v *Fusion7VM) IPAddress() (string, error) {
	if err := v.verifyVMXPath(); err != nil {
		return "", err
	}

	cmd := exec.Command(v.vmRunPath, "getGuestIPAddress", v.vmxPath, "-wait")
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
