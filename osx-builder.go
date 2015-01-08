package main

import (
	"log"
	"net/http"
	"strings"

	"github.com/c4milo/osx-builder/config"
	"github.com/c4milo/osx-builder/vms"
)

// Version string is injected when building the binary from Makefile.
var Version string

func init() {
	registry := map[string]map[string]func(http.ResponseWriter, *http.Request){
		"/vms": vms.Handlers,
	}

	http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		for p, handlers := range registry {
			if strings.HasPrefix(req.URL.Path, p) {
				if handlerFn, ok := handlers[req.Method]; ok {
					handlerFn(w, req)
					return
				}
				w.WriteHeader(http.StatusMethodNotAllowed)
				w.Write([]byte("Method Not Allowed"))
				return
			}
		}

		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Not Found"))
	})
}

func main() {
	address := ":" + config.Port
	err := http.ListenAndServe(address, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
