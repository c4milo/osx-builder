package vmware

import (
	"bytes"
	"fmt"
	"log"
	"os/exec"
	"strings"
)

// NetworkType represents all valid types of networking in VMware, except by "custom".
type NetworkType string

const (
	NetworkHostOnly NetworkType = "hostonly"
	NetworkNAT      NetworkType = "nat"
	NetworkBridged  NetworkType = "bridged"
)

// CloneType represents the type of clonning strategy when creating new VMs.
type CloneType string

const (
	CloneFull   CloneType = "full"
	CloneLinked CloneType = "linked"
)

// VMInfo defines the minimum amount of VM properties needed either to be configured
// or to be returned back for the purpose of this project.
type VMInfo struct {
	Name        string
	Annotation  string
	MemorySize  int
	CPUs        int
	GuestOS     string
	NetworkType NetworkType
}

// VirtualMachine defines the set of virtual machine operations used in this project.
type VirtualMachine interface {
	lookupVMRunPath() error
	Info() (*VMInfo, error)
	SetInfo(info *VMInfo) error
	CloneFrom(srcfile string, ctype CloneType) error
	Start(headless bool) error
	Stop() error
	Delete() error
	IsRunning() (bool, error)
	HasToolsInstalled() (bool, error)
	IPAddress() (string, error)
	Exists() (bool, error)
}

// Borrowed from https://github.com/mitchellh/packer/blob/master/builder/vmware/common/driver.go
func runAndLog(cmd *exec.Cmd) (string, string, error) {
	var stdout, stderr bytes.Buffer

	log.Printf("[VMWare] Executing: %s %v", cmd.Path, cmd.Args[1:])
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	stdoutString := strings.TrimSpace(stdout.String())
	stderrString := strings.TrimSpace(stderr.String())

	if _, ok := err.(*exec.ExitError); ok {
		message := stderrString
		if message == "" {
			message = stdoutString
		}

		err = fmt.Errorf("[VMWare] error: %s", message)
	}

	log.Printf("stdout: %s", stdoutString)
	log.Printf("stderr: %s", stderrString)

	// Replace these for Windows, we only want to deal with Unix
	// style line endings.
	returnStdout := strings.Replace(stdout.String(), "\r\n", "\n", -1)
	returnStderr := strings.Replace(stderr.String(), "\r\n", "\n", -1)

	return returnStdout, returnStderr, err
}
