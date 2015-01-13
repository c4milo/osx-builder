# OS X Builder

The OS X Builder is a small HTTP API that allows to run Go's buildlets on OS X virtual machines, running on VMWare Fusion.


## How it works
A VMware base image is provided upon VM creation. This image is cached and reused by the API as a "gold" or "pristine" copy. The gold image will be only downloaded and unpacked the first time the API receives a request for creating a virtual machine. From then on, and as long as the gold image is the same, the API is going to re-use that gold image to create linked clones from it. This is what the folder structure kept by this API looks like:

![](https://cldup.com/ekbdZPiZtc.png)

For more information about how linked clones work, please refer to the official documentation: https://www.vmware.com/support/ws55/doc/ws_clone_overview.html


## Caveats

* VMWare vmrun handles internal locking to avoid corruption of virtual machine files. If there is an attempt to get VM information when the VM is locked, you may get properties with empty values.

* In case you provide a callback URL, once it is called, there is not guarantee you will receive an IP Address as this will depend on IP acquisition timing as well as VMTools in the Guest OS taking its own time finding out the assigned IP address.

* When starting a VM in headless mode, the VM doesn't seem to boot in VMWare Fusion 7.
It does boot, though, if we start it with headless mode disabled. Some research was done and it seems to be an issue with VMWare Fusion itself, some Vagrant users have run into the same problem before but the real cause and fix hasn't been determined. 

# API
## HTTP response codes

* **202:** Request for creating a virtual machine was accepted
* **500:** Internal error
* **400:** Bad request
* **415:** The provided body data is not an accepted media type (application/json)
* **409:** Conflict when attempting to read virtual machine information. This could be due a stalled lock or a corrupt VMX file. Manual intervention may be needed.
* **404:** Virtual machine was not found

For errors, along with the HTTP response code, the API will return an error message as well. For example:

```json
{
  "code": "vm-not-found",
  "message": "The requested virtual machine ID was not found"
}
```

## Create virtual machine
Creates virtual machines asynchronously from an existing VMware virtual machine.

* **PATH:** `/vms`
* **Method:** `POST`
* **Consumes:** `application/json`
* **Produces:** `application/json`

**Body**

```json
{
	"cpus": 2,
	"memory": 1024,
	"network_type": "nat",
	"image": {
		"url": "https://github.com/hooklift/boxes/releases/download/coreos-dev-20141126/coreos_developer_vmware.tar.gz",
		"checksum": "5cf00d380e28d02f30efaceafef7c7c8bdedae33",
		"checksum_type": "sha1"
	},
	"headless": true,
	"callback_url": "http://foo.com/myscript",
}
```

**Valid network types:**

* bridged
* nat
* hostonly

Memory is understood in megabytes.


**Valid checksum algorithms:**

* md5
* sha1
* sha256
* sha512

**Supported compression formats for image packages:**

* tar.gz
* tar.bzip2
* zip
* tar

### Example

```shell
% curl -d@create.json http://localhost:12345/vms
{
  "id": "282ee68a-2e4d-4bd7-9c0e-7f37e12fc489",
  "image": {
    "url": "https://github.com/hooklift/boxes/releases/download/coreos-dev-20141126/coreos_developer_vmware.tar.gz",
    "checksum": "5cf00d380e28d02f30efaceafef7c7c8bdedae33",
    "checksum_type": "sha1"
  },
  "cpus": 2,
  "memory": 1024,
  "headless": true,
  "ip_address": "",
  "status": "",
  "guest_os": ""
}
```

Upon creation, you can either wait for your callback URL to be called by means of a POST method, or pull the VM information from time to time until you get back the rest of the information.

Once the creation process finishes, the following properties are going to be populated:

* ip_address
* status


## Retrieve virtual machine information
* **PATH:** `/vms/:id`
* **Method:** `GET`
* **Produces:** `application/json`

### Example

```shell
% curl http://localhost:12345/vms/c8a934d72293a7d31baf
{
  "id": "c8a934d72293a7d31baf",
  "image": {
    "url": "https://github.com/hooklift/boxes/releases/download/coreos-dev-20141126/coreos_developer_vmware.tar.gz",
    "checksum": "5cf00d380e28d02f30efaceafef7c7c8bdedae33",
    "checksum_type": "sha1"
  },
  "cpus": 2,
  "memory": 1024,
  "headless": false,
  "ip_address": "192.168.123.147",
  "status": "running",
}
```

## Destroy virtual machine
* **PATH:** `/vms/:id`
* **Method:** `DELETE`

### Example

```shell
% curl -X DELETE http://localhost:12345/vms/c8a934d72293a7d31baf
```

