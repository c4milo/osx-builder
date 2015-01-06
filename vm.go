package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"github.com/c4milo/unzipit"
	"github.com/dustin/go-humanize"
	govix "github.com/hooklift/govix"
)

// Virtual machine configuration
type VM struct {
	// Which VMware VIX service provider to use. ie: fusion, workstation, server, etc
	provider string
	// Whether to verify SSL or not for remote connections in ESXi
	verifySSL bool
	// Name of the virtual machine
	Name string `json:"id"`
	// Description for the virtual machine, it is created as an annotation in
	// VMware.
	description string
	// Image to use during the creation of this virtual machine
	Image Image `json:"image"`
	// Number of virtual cpus
	CPUs uint `json:"cpus"`
	// Memory size in megabytes.
	Memory string `json:"memory"`
	// Switches to where this machine is going to be attach to
	vSwitches []string
	// Whether to upgrade the VM virtual hardware
	upgradeVHardware bool
	// The timeout to wait for VMware Tools to be initialized inside the VM
	ToolsInitTimeout time.Duration `json:"tools_init_timeout"`
	// Whether to launch the VM with graphical environment
	LaunchGUI bool `json:"launch_gui"`
	// Whether to enable or disable shared folders for this VM
	sharedFolders bool
	// Network adapters
	vNetworkAdapters []*govix.NetworkAdapter
	// CD/DVD drives
	cDDVDDrives []*govix.CDDVDDrive
	// VM IP address as reported by VIX
	IPAddress string `json:"ip_address"`
	// Power status
	PowerState string `json:"power_state"`
	// Guest OS
	GuestOS string `json:"guest_os"`
}

// Creates VIX instance with VMware
func (v *VM) client() (*govix.Host, error) {
	var p govix.Provider

	switch strings.ToLower(v.provider) {
	case "fusion", "workstation":
		p = govix.VMWARE_WORKSTATION
	case "serverv1":
		p = govix.VMWARE_SERVER
	case "serverv2":
		p = govix.VMWARE_VI_SERVER
	case "player":
		p = govix.VMWARE_PLAYER
	case "workstation_shared":
		p = govix.VMWARE_WORKSTATION_SHARED
	default:
		p = govix.VMWARE_WORKSTATION
	}

	var options govix.HostOption
	if v.verifySSL {
		options = govix.VERIFY_SSL_CERT
	}

	host, err := govix.Connect(govix.ConnectConfig{
		Provider: p,
		Options:  options,
	})

	if err != nil {
		return nil, err
	}

	log.Printf("[INFO] VIX client configured for product: VMware %s. SSL: %t", v.provider, v.verifySSL)

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

	if v.description == "" {
		v.description = "Go's OSX Builder machine"
	}

	if v.ToolsInitTimeout.Seconds() <= 0 {
		v.ToolsInitTimeout = time.Duration(30) * time.Second
	}
}

// Downloads, extracts and opens Gold virtual machine, then it creates a clone
// out of it.
func (v *VM) Create() (string, error) {
	log.Printf("[DEBUG] Creating VM resource...")

	usr, err := user.Current()
	if err != nil {
		return "", err
	}

	image := v.Image
	goldPath := filepath.Join(usr.HomeDir, filepath.Join(".go-osx-builder/vix/gold", image.Checksum))
	_, err = os.Stat(goldPath)
	finfo, _ := ioutil.ReadDir(goldPath)
	goldPathEmpty := len(finfo) == 0

	if os.IsNotExist(err) || goldPathEmpty {
		log.Println("[DEBUG] Gold virtual machine does not exist or is empty")

		imgPath := filepath.Join(usr.HomeDir, ".go-osx-builder/vix/images", image.Checksum)
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

	baseVMDir := filepath.Join(usr.HomeDir, ".go-osx-builder", "vix", "vms",
		image.Checksum, v.Name)

	newvmx := filepath.Join(baseVMDir, v.Name+".vmx")

	if _, err = os.Stat(newvmx); os.IsNotExist(err) {
		log.Printf("[INFO] Virtual machine clone not found: %s, err: %+v", newvmx, err)
		// If there is not a VMX file, make sure nothing else is in there either.
		// We were seeing VIX 13004 errors when only a nvram file existed.
		os.RemoveAll(baseVMDir)

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
		err = v.powerOff(vm)
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

	log.Printf("[DEBUG] Setting name to %s", v.Name)
	vm.SetDisplayName(v.Name)

	log.Printf("[DEBUG] Setting description to %s", v.description)
	vm.SetAnnotation(v.description)

	if v.upgradeVHardware &&
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
	for _, adapter := range v.vNetworkAdapters {
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

	log.Printf("[DEBUG] Removing all CD/DVD drives from vmx file...")
	err = vm.RemoveAllCDDVDDrives()
	if err != nil {
		return err
	}

	log.Println("[INFO] Attaching CD/DVD drives... ")
	for _, cdrom := range v.cDDVDDrives {
		err := vm.AttachCDDVD(cdrom)
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

		if v.sharedFolders {
			log.Println("[WARN] Enabling shared folders is not possible.")
		}
		return nil
	}

	if v.sharedFolders {
		log.Println("[DEBUG] Enabling shared folders...")

		err = vm.EnableSharedFolders(v.sharedFolders)
		if err != nil {
			return err
		}
	}
	return nil
}

// Powers off a virtual machine attempting a graceful shutdown.
func (v *VM) powerOff(vm *govix.VM) error {
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
		if err = v.powerOff(vm); err != nil {
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

	return vm.Delete(govix.VMDELETE_KEEP_FILES | govix.VMDELETE_FORCE)
}

// Refreshes state with VMware
func (v *VM) Refresh(vmxFile string) error {
	log.Printf("[DEBUG] Syncing VM resource %s...", vmxFile)

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
	v.Name, err = vm.DisplayName()
	if err != nil {
		return err
	}

	v.description, err = vm.Annotation()
	if err != nil {
		return err
	}

	v.vNetworkAdapters, err = vm.NetworkAdapters()
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

	v.PowerState = string(powerState)
	v.GuestOS, err = vm.GuestOS()
	if err != nil {
		return err
	}

	return nil
}
