# OSX Builder Service

The OSX Builder is a small HTTP API that allows to create and destroy virtual machines in VMWare Fusion or Workstation.


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
	"memory": "1gib",
	"network_type": "nat",
	"image": {
		"url": "https://github.com/hooklift/boxes/releases/download/coreos-dev-20141126/coreos_developer_vmware.tar.gz",
		"checksum": "5cf00d380e28d02f30efaceafef7c7c8bdedae33",
		"checksum_type": "sha1"
	},
	"launch_gui": true,
	"callback_url": "http://foo.com/myscript",
}
```

**Valid network types:**

* bridged
* nat
* hostonly

Memory has to be specified in IEC units. Example: 1gib, 1024mib, 1048576kib


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
  "memory": "1024mib",
  "tools_init_timeout": 0,
  "launch_gui": true,
  "ip_address": "",
  "status": "",
  "guest_os": ""
}
```

Upon creation, you can either wait for your callback URL to be called by means of a POST method, or pull the VM information from time to time until you get back the rest of the information.

Once the creation process finishes, the following properties are going to be populated: 

* ip_address
* status
* guest_os


## Retrieve virtual machine information
* **PATH:** `/vms/:id`
* **Method:** `GET`
* **Produces:** `application/json`

### Example

```shell
% curl http://localhost:12345/vms/282ee68a-2e4d-4bd7-9c0e-7f37e12fc489
{
  "id": "282ee68a-2e4d-4bd7-9c0e-7f37e12fc489",
  "image": {
    "url": "https://github.com/hooklift/boxes/releases/download/coreos-dev-20141126/coreos_developer_vmware.tar.gz",
    "checksum": "5cf00d380e28d02f30efaceafef7c7c8bdedae33",
    "checksum_type": "sha1"
  },
  "cpus": 2,
  "memory": "1.0gib",
  "tools_init_timeout": 0,
  "launch_gui": false,
  "ip_address": "192.168.123.147",
  "status": "powered-on",
  "guest_os": "other26xlinux-64"
}
```

## Destroy virtual machine
* **PATH:** `/vms/:id`
* **Method:** `DELETE`

### Example

```shell
% curl -X DELETE http://localhost:12345/vms/282ee68a-2e4d-4bd7-9c0e-7f37e12fc489
```

## Caveats:
* There are sporadic crashes when creating virtual machines, this seems to be an issue within govix.

* VMWare VIX handles internal locking to avoid corruption of virtual machine files. If there is an attempt to get VM information when the VM is locked, you may get properties with empty values.

* In case you provide a callback URL, once it is called, there is not guarantee you will receive an IP Address as this will depend on IP acquisition timing.
