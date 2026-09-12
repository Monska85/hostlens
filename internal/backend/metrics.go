package backend

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/Monska85/hostlens/internal/telemetry"
)

// This operation is available only through the existing peer-checked IPC listener.
// It never enters diagnostic discovery or generation synchronization.
func (s *Server) telemetryHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), telemetry.BackendTimeout)
	defer cancel()
	deadline, _ := ctx.Deadline()
	controller := http.NewResponseController(w)
	for _, set := range []func(time.Time) error{controller.SetReadDeadline, controller.SetWriteDeadline} {
		if err := set(deadline); err != nil && !errors.Is(err, http.ErrNotSupported) {
			http.Error(w, "deadline unavailable", 500)
			return
		}
	}
	if !s.scrapes.Acquire() {
		http.Error(w, "telemetry overload", 503)
		return
	}
	reply, err := s.scrapes.Run(ctx, func() telemetry.Reply {
		families, err := s.metrics.Gather()
		if err != nil {
			return telemetry.Reply{Status: 500}
		}
		body, err := telemetry.Wire(families)
		if err != nil {
			return telemetry.Reply{Status: 500}
		}
		return telemetry.Reply{Body: body, Status: 200}
	})
	if err != nil {
		http.Error(w, "telemetry deadline exceeded", 503)
		return
	}
	if reply.Status != 200 {
		http.Error(w, "telemetry unavailable", reply.Status)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(reply.Body)
}
