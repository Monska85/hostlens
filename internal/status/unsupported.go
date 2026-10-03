//go:build !linux

package status

import (
	"context"
	"errors"
)

type unsupported struct{}

func New() Reader { return unsupported{} }

func (unsupported) Host(context.Context, func(string) bool) Host {
	return Host{Issues: []Issue{{Source: "host", Kind: "unsupported_platform"}}}
}

func (unsupported) Service(context.Context, string) (Service, error) {
	return Service{}, errors.New("service adapter unavailable on this platform")
}

func (unsupported) Services(context.Context, func(string) bool) (ServiceInventory, error) {
	return ServiceInventory{}, errors.New("service adapter unavailable on this platform")
}

func (unsupported) ServiceLogs(context.Context, string, int) (ServiceLog, error) {
	return ServiceLog{}, errors.New("journal adapter unavailable on this platform")
}
