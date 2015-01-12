package render

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// Options type to configure rendering
type Options struct {
	Status int
	Data   interface{}
	Cache  bool
}

// Renders JSON and send it to the HTTP client, supports caching
func JSON(w http.ResponseWriter, opts Options) error {
	if &w == nil {
		return fmt.Errorf("You must provide a http.ResponseWriter")
	}

	headers := w.Header()
	headers.Set("Content-Type", "application/json; charset=utf-8")

	if opts.Cache {
		headers.Set("Cache-Control", "no-cache, no-store, must-revalidate")
		headers.Set("Pragma", "no-cache")
		headers.Set("Expires", "0")
	}

	jsonBytes, err := json.Marshal(opts.Data)
	if err != nil {
		return err
	}

	headers.Set("Content-Length", strconv.Itoa(len(jsonBytes)))
	if opts.Status <= 0 {
		opts.Status = http.StatusOK
	}
	w.WriteHeader(opts.Status)
	w.Write(jsonBytes)

	return nil
}
