package main

import (
	"log"
	"net/http"
	"os"
)

var (
	Port string
)

func init() {
	Port = os.Getenv("PORT")
	if Port == "" {
		Port = "12345"
	}
}

func main() {
	http.HandleFunc("/vms", VMHandler)

	err := http.ListenAndServe(":"+Port, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

type BuilderError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Trace   string `json:"trace"`
}

func VMHandler(w http.ResponseWriter, r *http.Request) {
	var err error
	var vms []VMInfo
	var vminfo VMInfo

	switch r.Method {
	case "POST":
		vminfo, err = LaunchVM(w, r)
	case "DELETE":
		err = DestroyVM(w, r)
	case "GET":
		vms, err = ListVMs(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		w.Write([]byte("Method Not Allowed"))
	}

	if err != nil {
		switch err {
		default:
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
		}
	}
}

type OSImage struct {
	URL          string `json:"url"`
	Checksum     string `json:"checksum"`
	ChecksumType string `json:"checksum_type"`
}

type VMInfo struct {
	ID        string      `json:"id"`
	IPAddress string      `json:"ip_address"`
	Status    string      `json:"status"`
	GuestOS   string      `json:"guest_os"`
	NetType   NetworkType `json:"network_type"`
	CPUs      uint        `json:"cpus"`
	Memory    string      `json:"memory"`
	Image     OSImage     `json:"image"`
}

type NetworkType string

const (
	Bridged  NetworkType = "bridged"
	NAT      NetworkType = "nat"
	HostOnly NetworkType = "hostonly"
)

type LaunchVMParams struct {
	CPUs           uint        `json:"cpus"`
	Memory         string      `json:"memory"`
	NetType        NetworkType `json:"network_type"`
	Image          OSImage     `json:"image"`
	BoostrapScript string      `json:"boostrap_script"`
}

func LaunchVM(params LaunchVMParams) {

}

type DestroyVMParams struct {
	ID string
}

func DestroyVM(params DestroyVMParams) {

}

type ListVMsParams struct {
	Status string
}

func ListVMs(params ListVMsParams) {

}
