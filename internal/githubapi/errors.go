package githubapi

import (
	"errors"

	"github.com/cli/go-gh/v2/pkg/api"
)

// StatusCode extracts the HTTP status from a client error, or 0.
func StatusCode(err error) int {
	var stub *HTTPError
	if errors.As(err, &stub) {
		return stub.StatusCode
	}
	var gh *api.HTTPError
	if errors.As(err, &gh) {
		return gh.StatusCode
	}
	return 0
}
