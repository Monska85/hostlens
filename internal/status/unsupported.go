//go:build !linux

package status

import (
	"context"
	"errors"
)

type unsupported struct{}

func New() Reader { return unsupported{} }

func (unsupported) Host(context.Context) Host {
	return Host{Issues: []Issue{{Source: "host", Kind: "unsupported_platform"}}}
}

func (unsupported) Service(context.Context, string) (Service, error) {
	return Service{}, errors.New("service adapter unavailable on this platform")
}
