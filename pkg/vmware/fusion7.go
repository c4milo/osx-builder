package vmware

import (
	"errors"
	"fmt"
	"os"
)

type Fusion7VM struct {
	vmxpath string
}

func NewFusion7VM(vmxpath string) *Fusion7VM {
	return &Fusion7VM{
		vmxpath: vmxpath,
	}
}

func (v *Fusion7VM) vmrunPath() (string, error) {
	vmrun := os.Getenv("VMWARE_VMRUN_PATH")

	if vmrun != "" {
		return vmrun, nil
	}

	// Guess it is in the default installation path
	vmrun = "/Applications/VMware Fusion.app/Contents/Library/vmrun"
	if _, err := os.Stat(vmrun); err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("VMWare vmrun not found at path: %s", vmrun)
		}
	}

	return vmrun, nil
}

func (v *Fusion7VM) verifyVMXPath() error {
	if v.vmxpath == "" {
		return errors.New("[Fusion7] Empty VMX file path. Nothing to operate on.")
	}
	return nil
}

func (v *Fusion7VM) Info() (*VMInfo, error) {
	if err := v.verifyVMXPath(); err != nil {
		return nil, err
	}
	return nil, nil
}

func (v *Fusion7VM) SetInfo(info *VMInfo) error {
	if err := v.verifyVMXPath(); err != nil {
		return err
	}
	return nil
}

func (v *Fusion7VM) CloneFrom(srcfile string, ctype CloneType) error {
	if err := v.verifyVMXPath(); err != nil {
		return err
	}
	return nil
}

func (v *Fusion7VM) Start(gui bool) error {
	if err := v.verifyVMXPath(); err != nil {
		return err
	}
	return nil
}

func (v *Fusion7VM) Stop() error {
	if err := v.verifyVMXPath(); err != nil {
		return err
	}
	return nil
}

func (v *Fusion7VM) Delete() error {
	if err := v.verifyVMXPath(); err != nil {
		return err
	}
	return nil
}

func (v *Fusion7VM) IsRunning() (bool, error) {
	if err := v.verifyVMXPath(); err != nil {
		return false, err
	}
	return false, nil
}

func (v *Fusion7VM) HasToolsInstalled() (bool, error) {
	if err := v.verifyVMXPath(); err != nil {
		return false, err
	}
	return false, nil
}

func (v *Fusion7VM) IPAddress() (string, error) {
	if err := v.verifyVMXPath(); err != nil {
		return "", err
	}
	return "", nil
}

func (v *Fusion7VM) Exists() (bool, error) {
	if err := v.verifyVMXPath(); err != nil {
		return false, err
	}

	if _, err := os.Stat(v.vmxpath); os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}

	return true, nil
}
