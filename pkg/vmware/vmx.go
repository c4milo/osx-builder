// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vmware

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"sort"
	"strings"
)

// readVMX reads data from the given VMX file path.
func readVMXFile(vmxpath string) (map[string]string, error) {
	data, err := ioutil.ReadFile(vmxpath)
	if err != nil {
		return nil, err
	}

	vmx := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		values := strings.Split(line, "=")
		if len(values) != 2 {
			continue
		}

		k := strings.TrimSpace(values[0])
		v := strings.TrimSpace(values[1])
		vmx[strings.ToLower(k)] = strings.Trim(v, `"`)
	}

	return vmx, nil
}

// writeVMX writes to the provided VMX file path from data provided in a map.
func writeVMXFile(vmxpath string, vmx map[string]string) error {
	f, err := os.Create(vmxpath)
	if err != nil {
		return err
	}

	defer f.Close()

	i := 0
	keys := make([]string, len(vmx))
	for k := range vmx {
		keys[i] = k
		i++
	}

	sort.Strings(keys)

	var buf bytes.Buffer
	for _, key := range keys {
		buf.WriteString(key + " = " + `"` + vmx[key] + `"`)
		buf.WriteString("\n")
	}

	if _, err = io.Copy(f, &buf); err != nil {
		return err
	}

	return nil
}
