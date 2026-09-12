package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Monska85/hostlens/internal/telemetry"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
)

func metricsSmoke(endpoint, credential, expect string) error {
	secret := ""
	if credential == "invalid" {
		secret = "invalid"
	} else if credential != "anonymous" {
		b, err := os.ReadFile(credential)
		if err != nil {
			return err
		}
		var token struct{ Secret string }
		if err = json.Unmarshal(b, &token); err != nil {
			return err
		}
		secret = token.Secret
	}
	req, err := http.NewRequest("GET", strings.TrimSuffix(endpoint, "/mcp")+"/metrics", nil)
	if err != nil {
		return err
	}
	if secret != "" {
		req.Header.Set("Authorization", "Bearer "+secret)
	}
	client := http.Client{Timeout: 6 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if status, err := strconv.Atoi(expect); err == nil {
		if resp.StatusCode != status {
			return fmt.Errorf("metrics status: got %d want %d", resp.StatusCode, status)
		}
		return nil
	}
	if resp.StatusCode != 200 || resp.Header.Get("Content-Type") != telemetry.ContentType {
		return fmt.Errorf("metrics exposition HTTP %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, telemetry.MaxBytes+1))
	if err != nil || len(b) > telemetry.MaxBytes {
		return fmt.Errorf("metrics size/read failure")
	}
	parser := expfmt.NewTextParser(model.LegacyValidation)
	families, err := parser.TextToMetricFamilies(bytes.NewReader(b))
	if err != nil {
		return err
	}
	up := families["hostlens_backend_up"]
	if up == nil || len(up.Metric) != 1 {
		return fmt.Errorf("missing backend state")
	}
	want := 1.
	if expect == "down" {
		want = 0
	}
	if up.Metric[0].Gauge.GetValue() != want {
		return fmt.Errorf("unexpected backend state")
	}
	components := map[string]bool{}
	backendCalls := 0.
	gatewayCalls := 0.
	for name, f := range families {
		for _, m := range f.Metric {
			component := ""
			for _, l := range m.Label {
				if l.GetName() == "component" {
					component = l.GetValue()
					components[component] = true
				}
			}
			if name == "hostlens_tool_calls_total" {
				if component == "backend" {
					backendCalls += m.Counter.GetValue()
				}
				if component == "gateway" {
					gatewayCalls += m.Counter.GetValue()
				}
			}
		}
	}
	if !components["gateway"] || (expect == "down" && components["backend"]) || (expect != "down" && !components["backend"]) {
		return fmt.Errorf("unexpected component observations")
	}
	if expect == "fresh" && (backendCalls != 0 || gatewayCalls == 0) {
		return fmt.Errorf("backend restart did not reset only backend counters")
	}
	return nil
}
