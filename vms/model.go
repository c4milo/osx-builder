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
	"strings"
	"time"

	"github.com/c4milo/go-osx-builder/config"
	"github.com/c4milo/unzipit"
	"github.com/dustin/go-humanize"
	govix "github.com/hooklift/govix"
)

// Virtual machine configuration
type VM struct {
	// VMX file path for Internal use only
	VMXFile string `json:"-"`
	// Which VMware VIX service provider to use. ie: fusion, workstation, server, etc
	Provider govix.Provider `json:"-"`
	// Whether to verify SSL or not for remote connections in ESXi
	VerifySSL bool `json:"-"`
	// ID of the virtual machine
	ID string `json:"id"`
	// Image to use during the creation of this virtual machine
	Image Image `json:"image"`
	// Number of virtual cpus
	CPUs uint `json:"cpus"`
	// Memory size in megabytes.
	Memory string `json:"memory"`
	// Whether to upgrade the VM virtual hardware
	UpgradeVHardware bool `json:"-"`
	// The timeout to wait for VMware Tools to be initialized inside the VM
	ToolsInitTimeout time.Duration `json:"tools_init_timeout"`
	// Whether to launch the VM with graphical environment
	LaunchGUI bool `json:"launch_gui"`
	// Network adapters
	VNetworkAdapters []*govix.NetworkAdapter `json:"-"`
	// VM IP address as reported by VIX
	IPAddress string `json:"ip_address"`
	// Power status
	Status string `json:"status"`
	// Guest OS
	GuestOS string `json:"guest_os"`
}

// Creates VIX instance with VMware Fusion/Workstation, returning a handle
// to a VIX Host
func (v *VM) client() (*govix.Host, error) {
	var options govix.HostOption
	if v.VerifySSL {
		options = govix.VERIFY_SSL_CERT
	}

	host, err := govix.Connect(govix.ConnectConfig{
		Provider: v.Provider,
		Options:  options,
	})

	if err != nil {
		return nil, err
	}

	log.Printf("[INFO] VIX client configured for product: VMware %d. SSL: %t", v.Provider, v.VerifySSL)

	return host, nil
}

// Sets default values for VM attributes
func (v *VM) SetDefaults() {
	if v.CPUs <= 0 {
		v.CPUs = 2
	}

	if v.Memory == "" {
		v.Memory = "512mib"
	}

	if v.ToolsInitTimeout.Seconds() <= 0 {
		v.ToolsInitTimeout = time.Duration(30) * time.Second
	}
}

// Downloads OS image, creates and launches a virtual machine.
func (v *VM) Create() (string, error) {
	log.Printf("[DEBUG] Creating VM resource...")

	image := v.Image
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

		log.Printf("[DEBUG] Unpacking Gold virtual machine into %s\n", goldPath)
		_, err = unzipit.Unpack(image.file, goldPath)
		if err != nil {
			debug.PrintStack()
			log.Printf("[ERROR] Unpacking Gold image %s\n", image.file.Name())
			return "", err
		}
	}

	pattern := filepath.Join(goldPath, "**.vmx")

	log.Printf("[DEBUG] Finding Gold virtual machine vmx file in %s", pattern)
	files, _ := filepath.Glob(pattern)

	if len(files) == 0 {
		return "", fmt.Errorf("[ERROR] vmx file was not found: %s", pattern)
	}

	vmxFile := files[0]
	log.Printf("[DEBUG] Gold virtual machine vmx file found %v", vmxFile)

	// Gets VIX instance
	client, err := v.client()
	if err != nil {
		return "", err
	}
	defer client.Disconnect()

	log.Printf("[INFO] Opening Gold virtual machine from %s", vmxFile)

	vm, err := client.OpenVM(vmxFile, v.Image.Password)
	if err != nil {
		return "", err
	}

	vmFolder := filepath.Join(config.VMSPath, v.ID)
	newvmx := filepath.Join(config.VMSPath, v.ID, v.ID+".vmx")

	if _, err = os.Stat(newvmx); os.IsNotExist(err) {
		log.Printf("[INFO] Virtual machine clone not found: %s, err: %+v", newvmx, err)
		// If there is not a VMX file, make sure nothing else is in there either.
		// We were seeing VIX 13004 errors when only a nvram file existed.
		os.RemoveAll(vmFolder)

		log.Printf("[INFO] Cloning Gold virtual machine into %s...", newvmx)
		_, err := vm.Clone(govix.CLONETYPE_FULL, newvmx)

		// If there is an error and the error is other than "The snapshot already exists"
		// then return the error
		if err != nil && err.(*govix.Error).Code != 13004 {
			return "", err
		}
	} else {
		log.Printf("[INFO] Virtual Machine clone %s already exist, moving on.", newvmx)
	}

	if err = v.Update(newvmx); err != nil {
		return "", err
	}

	return newvmx, nil
}

// Opens and updates virtual machine resource
func (v *VM) Update(vmxFile string) error {
	// Sets default values if some attributes were not set or have
	// invalid values
	v.SetDefaults()

	// Gets VIX instance
	client, err := v.client()
	if err != nil {
		return err
	}
	defer client.Disconnect()

	if client.Provider == govix.VMWARE_VI_SERVER ||
		client.Provider == govix.VMWARE_SERVER {
		log.Printf("[INFO] Registering VM in host's inventory...")
		err = client.RegisterVM(vmxFile)
		if err != nil {
			return err
		}
	}

	log.Printf("[INFO] Opening virtual machine from %s", vmxFile)

	vm, err := client.OpenVM(vmxFile, v.Image.Password)
	if err != nil {
		return err
	}

	running, err := vm.IsRunning()
	if err != nil {
		return err
	}

	if running {
		log.Printf("[INFO] Virtual machine seems to be running, we need to " +
			"power it off in order to make changes.")
		err = powerOff(vm)
		if err != nil {
			return err
		}
	}

	memoryInMb, err := humanize.ParseBytes(v.Memory)
	if err != nil {
		log.Printf("[WARN] Unable to set memory size, defaulting to 512mib: %s", err)
		memoryInMb = 512
	} else {
		memoryInMb = (memoryInMb / 1024) / 1024
	}

	log.Printf("[DEBUG] Setting memory size to %d megabytes", memoryInMb)
	vm.SetMemorySize(uint(memoryInMb))

	log.Printf("[DEBUG] Setting vcpus to %d", v.CPUs)
	vm.SetNumberVcpus(v.CPUs)

	log.Printf("[DEBUG] Setting ID to %s", v.ID)
	vm.SetDisplayName(v.ID)

	imageJSON, err := json.Marshal(v.Image)
	if err != nil {
		return err
	}

	// We need to encode the JSON data in base64 so that the VMX file is not
	// interpreted by VMWare as corrupted.
	vm.SetAnnotation(base64.StdEncoding.EncodeToString(imageJSON))

	if v.UpgradeVHardware &&
		client.Provider != govix.VMWARE_PLAYER {

		log.Println("[INFO] Upgrading virtual hardware...")
		err = vm.UpgradeVHardware()
		if err != nil {
			return err
		}
	}

	log.Printf("[DEBUG] Removing all network adapters from vmx file...")
	err = vm.RemoveAllNetworkAdapters()
	if err != nil {
		return err
	}

	log.Println("[INFO] Attaching virtual network adapters...")
	for _, adapter := range v.VNetworkAdapters {
		adapter.StartConnected = true
		if adapter.ConnType == govix.NETWORK_BRIDGED {
			adapter.LinkStatePropagation = true
		}

		log.Printf("[DEBUG] Adapter: %+v", adapter)
		err := vm.AddNetworkAdapter(adapter)
		if err != nil {
			return err
		}
	}

	log.Println("[INFO] Powering virtual machine on...")
	var options govix.VMPowerOption

	if v.LaunchGUI {
		log.Println("[INFO] Preparing to launch GUI...")
		options |= govix.VMPOWEROP_LAUNCH_GUI
	}

	options |= govix.VMPOWEROP_NORMAL

	err = vm.PowerOn(options)
	if err != nil {
		return err
	}

	log.Printf("[INFO] Waiting %s for VMware Tools to initialize...\n", v.ToolsInitTimeout)
	err = vm.WaitForToolsInGuest(v.ToolsInitTimeout)
	if err != nil {
		log.Println("[WARN] VMware Tools took too long to initialize or is not " +
			"installed.")
	}

	return nil
}

// Powers off a virtual machine attempting a graceful shutdown.
func powerOff(vm *govix.VM) error {
	tstate, err := vm.ToolsState()
	if err != nil {
		return err
	}

	var powerOpts govix.VMPowerOption
	log.Printf("Tools state %d", tstate)

	if (tstate & govix.TOOLSSTATE_RUNNING) != 0 {
		log.Printf("[INFO] VMware Tools is running, attempting a graceful shutdown...")
		// if VMware Tools is running, attempt a graceful shutdown.
		powerOpts |= govix.VMPOWEROP_FROM_GUEST
	} else {
		log.Printf("[INFO] VMware Tools is NOT running, shutting down the " +
			"machine abruptly...")
		powerOpts |= govix.VMPOWEROP_NORMAL
	}

	err = vm.PowerOff(powerOpts)
	if err != nil {
		return err
	}
	log.Printf("[DEBUG] Virtual machine is off.")

	return nil
}

// Destroys a virtual machine resource
func (v *VM) Destroy(vmxFile string) error {
	log.Printf("[DEBUG] Destroying VM resource %s...", vmxFile)

	client, err := v.client()
	if err != nil {
		return err
	}
	defer client.Disconnect()

	vm, err := client.OpenVM(vmxFile, v.Image.Password)
	if err != nil {
		return err
	}

	running, err := vm.IsRunning()
	if err != nil {
		return err
	}

	if running {
		if err = powerOff(vm); err != nil {
			return err
		}
	}

	if client.Provider == govix.VMWARE_VI_SERVER ||
		client.Provider == govix.VMWARE_SERVER {
		log.Printf("[INFO] Unregistering VM from host's inventory...")

		err := client.UnregisterVM(vmxFile)
		if err != nil {
			return err
		}
	}

	log.Println("[DEBUG] Asking VIX to delete the VM...")
	err = vm.Delete(govix.VMDELETE_KEEP_FILES | govix.VMDELETE_FORCE)
	if err != nil {
		return err
	}

	if v.ID != "" {
		os.RemoveAll(filepath.Join(config.VMSPath, v.ID))
	}

	log.Printf("[DEBUG] VM %s Destroyed.\n", vmxFile)

	return nil
}

// Finds a virtual machine by ID
func FindVM(id string) (*VM, error) {
	vmxFile := filepath.Join(config.VMSPath, id, id+".vmx")

	_, err := os.Stat(vmxFile)
	if os.IsNotExist(err) {
		return nil, nil
	}

	vm := &VM{}

	log.Printf("Getting VM information from %s...\n", vmxFile)
	err = vm.Refresh(vmxFile)
	if err != nil {
		return nil, err
	}

	return vm, nil
}

// Refreshes state with VMware
func (v *VM) Refresh(vmxFile string) error {
	log.Printf("[DEBUG] Syncing VM resource %s...", vmxFile)

	v.Provider = govix.VMWARE_WORKSTATION
	v.VerifySSL = false
	v.VMXFile = vmxFile

	client, err := v.client()
	if err != nil {
		return err
	}
	defer client.Disconnect()

	log.Printf("[DEBUG] Opening VM %s...", vmxFile)
	vm, err := client.OpenVM(vmxFile, v.Image.Password)
	if err != nil {
		return err
	}

	running, err := vm.IsRunning()
	if !running {
		return err
	}

	vcpus, err := vm.Vcpus()
	if err != nil {
		return err
	}

	memory, err := vm.MemorySize()
	if err != nil {
		return err
	}

	// We need to convert memory value to megabytes so humanize can interpret it
	// properly.
	memory = (memory * 1024) * 1024
	v.Memory = strings.ToLower(humanize.IBytes(uint64(memory)))
	v.CPUs = uint(vcpus)

	v.ID, err = vm.DisplayName()
	if err != nil {
		return err
	}

	imageJSONBase64, err := vm.Annotation()
	if err != nil {
		return err
	}

	imageJSON, err := base64.StdEncoding.DecodeString(imageJSONBase64)
	if err != nil {
		return err
	}

	var image Image
	err = json.Unmarshal(imageJSON, &image)
	if err != nil {
		return err
	}
	v.Image = image

	v.VNetworkAdapters, err = vm.NetworkAdapters()
	if err != nil {
		return err
	}

	v.IPAddress, err = vm.IPAddress()
	if err != nil {
		return err
	}

	powerState, err := vm.PowerState()
	if err != nil {
		return err
	}

	v.Status = Status(powerState)
	v.GuestOS, err = vm.GuestOS()
	if err != nil {
		return err
	}

	log.Printf("[DEBUG] Finished syncing VM %s...", vmxFile)
	return nil
}

// Resolves power state bitwise flags to more user friendly strings
func Status(s govix.VMPowerState) string {
	blockedOnMsg := ",blocked"
	toolsRunning := ",tools-running"
	status := "unknown"

	if (s & govix.POWERSTATE_POWERING_OFF) != 0 {
		status = "powering-off"
	}

	if (s & govix.POWERSTATE_POWERED_OFF) != 0 {
		status = "powered-off"
	}

	if (s & govix.POWERSTATE_POWERING_ON) != 0 {
		status = "powering-on"
	}

	if (s & govix.POWERSTATE_POWERED_ON) != 0 {
		status = "powered-on"
	}

	if (s & govix.POWERSTATE_SUSPENDING) != 0 {
		status = "suspending"
	}

	if (s & govix.POWERSTATE_SUSPENDED) != 0 {
		status = "suspended"
	}

	if (s & govix.POWERSTATE_RESETTING) != 0 {
		status = "resetting"
	}

	if (s & govix.POWERSTATE_TOOLS_RUNNING) != 0 {
		status += toolsRunning
	}

	if (s & govix.POWERSTATE_BLOCKED_ON_MSG) != 0 {
		status += blockedOnMsg
	}

	return status
}
