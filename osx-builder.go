// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

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

func main() {
	// Keeps a registry of path function handlers.
	registry := map[string]map[string]func(http.ResponseWriter, *http.Request){
		"/vms": vms.Handlers,
	}

	// Main entry point to handle requests. Based on a URL path, this piece of code
	// iterates the registry and invokes the path's function handler if there is
	// match.
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

	address := ":" + config.Port
	log.Fatal(http.ListenAndServe(address, nil))
}
