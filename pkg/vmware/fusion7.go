package vmware

import (
	"fmt"
	"os"
)

type Fusion7 struct{}

func (v *Fusion7) vmrunPath() (string, error) {
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

func (v *Fusion7) Info(vmxfile string) (*VMInfo, error) {
	return nil, nil
}

func (v *Fusion7) SetInfo(info *VMInfo) error {
	return nil
}

func (v *Fusion7) Clone(vmxfile, dstfile string, ctype CloneType) error {
	return nil
}

func (v *Fusion7) Start(vmxfile string, gui bool) error {
	return nil
}

func (v *Fusion7) Stop(vmxfile string) error {
	return nil
}

func (v *Fusion7) Delete(vmxfile string) error {
	return nil
}

func (v *Fusion7) IsRunning(vmxfile string) (bool, error) {
	return false, nil
}

func (v *Fusion7) HasToolsInstalled(vmxfile string) (bool, error) {
	return false, nil
}

func (v *Fusion7) IPAddress(vmxfile string) (string, error) {
	return "", nil
}
