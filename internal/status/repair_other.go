//go:build !linux

package status

import (
	"context"
	"errors"
)

type unsupportedRestarter struct{}

func NewRestarter() ServiceRestarter { return unsupportedRestarter{} }

func (unsupportedRestarter) RestartService(context.Context, string) (RestartOutcome, error) {
	return RestartOutcome{}, errors.New("service restart unavailable on this platform")
}
