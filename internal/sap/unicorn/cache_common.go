//go:build !ios

package unicorn

import (
	"net/http"
	"time"
)

var artifactHTTPClient = &http.Client{Timeout: 2 * time.Minute}

type runtimePaths struct {
	library      string
	dependencies []string
}
