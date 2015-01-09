package vmware

import (
	"bytes"
	"fmt"
	"log"
	"os/exec"
	"strings"
)

type NetworkType string

const (
	NetworkHostOnly NetworkType = "hostonly"
	NetworkNAT      NetworkType = "nat"
	NetworkBridged  NetworkType = "bridged"
)

type CloneType string

const (
	CloneFull   CloneType = "full"
	CloneLinked CloneType = "linked"
)

type NetworkAdapter struct {
	NetType NetworkType
}

type VMInfo struct {
	Name            string
	Annotation      string
	MemorySize      int
	CPUs            int
	NetworkAdapters []NetworkAdapter
}

type VMManager interface {
	vmrunPath() (string, error)
	Info(vmxfile string) (*VMInfo, error)
	SetInfo(info *VMInfo) error
	Clone(vmxfile, dstfile string, ctype CloneType) error
	Start(vmxfile string, gui bool) error
	Stop(vmxfile string) error
	Delete(vmxfile string) error
	IsRunning(vmxfile string) (bool, error)
	HasToolsInstalled(vmxfile string) (bool, error)
	IPAddress(vmxfile string) (string, error)
}

// Borrowed from https://github.com/mitchellh/packer/blob/master/builder/vmware/common/driver.go
func runAndLog(cmd *exec.Cmd) (string, string, error) {
	var stdout, stderr bytes.Buffer

	log.Printf("Executing: %s %v", cmd.Path, cmd.Args[1:])
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

		err = fmt.Errorf("VMware error: %s", message)
	}

	log.Printf("stdout: %s", stdoutString)
	log.Printf("stderr: %s", stderrString)

	// Replace these for Windows, we only want to deal with Unix
	// style line endings.
	returnStdout := strings.Replace(stdout.String(), "\r\n", "\n", -1)
	returnStderr := strings.Replace(stderr.String(), "\r\n", "\n", -1)

	return returnStdout, returnStderr, err
}
