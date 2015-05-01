package templar

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/amir/raidman"
)

type DebugStats struct{}

func (d *DebugStats) StartRequest(req *http.Request) {
	fmt.Printf("[%s] S %s %s\n", time.Now().Format(time.RFC3339Nano), req.Method, req.URL)
}

func (d *DebugStats) Emit(req *http.Request, dur time.Duration) {
	fmt.Printf("[%s] E %s %s (%s)\n", time.Now().Format(time.RFC3339Nano), req.Method, req.URL, dur)
}

func (d *DebugStats) RequestTimeout(req *http.Request, timeout time.Duration) {
	fmt.Printf("[%s] T %s %s (%s)\n", time.Now().Format(time.RFC3339Nano), req.Method, req.URL, timeout)
}

var _ = Stats(&DebugStats{})

type StatsdOutput struct {
	client StatsdClient
}

var _ = Stats(&StatsdOutput{})

func NewStatsdOutput(client StatsdClient) *StatsdOutput {
	return &StatsdOutput{client}
}

func (s *StatsdOutput) url(req *http.Request) string {
	return req.Host + strings.Replace(req.URL.Path, "/", "-", -1)
}

func (s *StatsdOutput) StartRequest(req *http.Request) {
	s.client.Incr("templar.request.method."+req.Method, 1)
	s.client.Incr("templar.request.host."+req.Host, 1)
	s.client.Incr("templar.request.url."+s.url(req), 1)
	s.client.GaugeDelta("templar.requests.active", 1)
}

func (s *StatsdOutput) Emit(req *http.Request, delta time.Duration) {
	s.client.GaugeDelta("templar.requests.active", -1)
	s.client.PrecisionTiming("templar.request.url."+s.url(req), delta)
}

func (s *StatsdOutput) RequestTimeout(req *http.Request, timeout time.Duration) {
	s.client.Incr("templar.timeout.host."+req.Host, 1)
	s.client.Incr("templar.timeout.url."+s.url(req), 1)
}

type RiemannOutput struct {
	client RiemannClient
}

func NewRiemannOutput(client RiemannClient) *RiemannOutput {
	return &RiemannOutput{client}
}

func (r *RiemannOutput) StartRequest(req *http.Request) {
	attributes := make(map[string]string)
	attributes["method"] = req.Method
	attributes["host"] = req.Host
	attributes["path"] = req.URL.Path
	var event = &raidman.Event{
		State:      "ok",
		Service:    "templar request",
		Metric:     1,
		Attributes: attributes,
	}
	r.client.Send(event)
}

func (r *RiemannOutput) Emit(req *http.Request, delta time.Duration) {
	attributes := make(map[string]string)
	attributes["method"] = req.Method
	attributes["host"] = req.Host
	attributes["path"] = req.URL.Path
	var event = &raidman.Event{
		State:      "ok",
		Service:    "templar response",
		Metric:     1000.0 * delta.Seconds(),
		Attributes: attributes,
	}
	r.client.Send(event)
}

func (r *RiemannOutput) RequestTimeout(req *http.Request, timeout time.Duration) {
	attributes := make(map[string]string)
	attributes["method"] = req.Method
	attributes["host"] = req.Host
	attributes["path"] = req.URL.Path
	var event = &raidman.Event{
		State:      "warning",
		Service:    "templar timeout",
		Metric:     timeout.Seconds() * 1000.0,
		Attributes: attributes,
	}
	r.client.Send(event)
}

type MultiStats []Stats

var _ = Stats(MultiStats{})

func (m MultiStats) StartRequest(req *http.Request) {
	for _, s := range m {
		s.StartRequest(req)
	}
}

func (m MultiStats) Emit(req *http.Request, t time.Duration) {
	for _, s := range m {
		s.Emit(req, t)
	}
}

func (m MultiStats) RequestTimeout(req *http.Request, timeout time.Duration) {
	for _, s := range m {
		s.RequestTimeout(req, timeout)
	}
}
