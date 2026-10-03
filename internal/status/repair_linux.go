//go:build linux

package status

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/coreos/go-systemd/v22/dbus"
)

// LinuxRestarter is composed only into the separate repair process.
type LinuxRestarter struct{}

func known(value bool) *bool { return &value }

func NewRestarter() ServiceRestarter { return LinuxRestarter{} }

func (LinuxRestarter) RestartService(ctx context.Context, name string) (RestartOutcome, error) {
	if err := validateUnit(name); err != nil {
		return RestartOutcome{}, err
	}
	conn, err := dbus.NewSystemConnectionContext(ctx)
	if err != nil {
		return RestartOutcome{}, fmt.Errorf("systemd unavailable: %w", err)
	}
	defer conn.Close()
	properties, err := conn.GetUnitPropertiesContext(ctx, name)
	if err != nil {
		return RestartOutcome{}, fmt.Errorf("service inspection failed: %w", err)
	}
	identity, _ := properties["Id"].(string)
	loaded, _ := properties["LoadState"].(string)
	if identity != name || loaded != "loaded" {
		return RestartOutcome{}, errors.New("service identity or loaded state changed")
	}
	completed := make(chan string, 1)
	if _, err := conn.RestartUnitContext(ctx, name, "replace", completed); err != nil {
		return RestartOutcome{Issue: "restart_outcome_unknown"}, nil
	}
	select {
	case outcome := <-completed:
		if outcome != "done" {
			return RestartOutcome{Invoked: known(true), Completed: known(false), Issue: "restart_failed"}, nil
		}
	case <-ctx.Done():
		return RestartOutcome{Invoked: known(true), Issue: "restart_outcome_unknown"}, nil
	}
	after, err := conn.GetUnitPropertiesContext(ctx, name)
	if err != nil {
		return RestartOutcome{Invoked: known(true), Completed: known(true), Issue: "post_observation_unavailable"}, nil
	}
	postID, _ := after["Id"].(string)
	if postID != name {
		return RestartOutcome{Invoked: known(true), Completed: known(true), Issue: "post_identity_changed"}, nil
	}
	property := func(key string) string {
		value, _ := after[key].(string)
		return value
	}
	service := Service{ObservedAt: time.Now().UTC(), Name: name, Availability: property("LoadState"), State: property("ActiveState"), Detail: property("SubState")}
	return RestartOutcome{Service: &service, Invoked: known(true), Completed: known(true)}, nil
}
