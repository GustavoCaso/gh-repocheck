// Package githubapi abstracts the GitHub REST client so checks are testable.
package githubapi

import (
	"context"
	"io"
	"net/http"

	"github.com/cli/go-gh/v2/pkg/api"
)

// Client is the context-aware subset of GitHub REST operations that checks use.
type Client interface {
	Get(ctx context.Context, path string, response any) error
	Post(ctx context.Context, path string, body io.Reader, response any) error
	Patch(ctx context.Context, path string, body io.Reader, response any) error
	Put(ctx context.Context, path string, body io.Reader, response any) error
}

// GH adapts go-gh's RESTClient to Client, propagating context.
type GH struct{ REST *api.RESTClient }

var _ Client = (*GH)(nil)

func (c *GH) Get(ctx context.Context, path string, response any) error {
	return c.REST.DoWithContext(ctx, http.MethodGet, path, nil, response)
}

func (c *GH) Post(ctx context.Context, path string, body io.Reader, response any) error {
	return c.REST.DoWithContext(ctx, http.MethodPost, path, body, response)
}

func (c *GH) Patch(ctx context.Context, path string, body io.Reader, response any) error {
	return c.REST.DoWithContext(ctx, http.MethodPatch, path, body, response)
}

func (c *GH) Put(ctx context.Context, path string, body io.Reader, response any) error {
	return c.REST.DoWithContext(ctx, http.MethodPut, path, body, response)
}
