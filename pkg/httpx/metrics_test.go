package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
)

type fakeObserver struct {
	observed    []observedRequest
	inFlight    int
	maxInFlight int
}

type observedRequest struct {
	method   string
	route    string
	status   int
	duration time.Duration
}

func (f *fakeObserver) ObserveHTTP(method, route string, status int, d time.Duration) {
	f.observed = append(f.observed, observedRequest{method, route, status, d})
}

func (f *fakeObserver) IncInFlight() {
	f.inFlight++
	if f.inFlight > f.maxInFlight {
		f.maxInFlight = f.inFlight
	}
}

func (f *fakeObserver) DecInFlight() { f.inFlight-- }

func TestMetrics_RecordsRoutePatternAndStatus(t *testing.T) {
	obs := &fakeObserver{}
	r := chi.NewRouter()
	r.Use(Metrics(obs))
	r.Get("/api/v1/events/{eventID}", func(w http.ResponseWriter, req *http.Request) {
		time.Sleep(2 * time.Millisecond)
		Success(w, http.StatusOK, map[string]string{"id": "1"})
	})

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/events/abc", nil))

	if len(obs.observed) != 1 {
		t.Fatalf("expected one observation, got %d", len(obs.observed))
	}
	got := obs.observed[0]
	if got.method != http.MethodGet || got.route != "/api/v1/events/{eventID}" || got.status != http.StatusOK {
		t.Fatalf("unexpected observation: %+v", got)
	}
	if got.duration <= 0 {
		t.Fatalf("expected positive duration, got %v", got.duration)
	}
	if obs.inFlight != 0 {
		t.Fatalf("in-flight should return to zero, got %d", obs.inFlight)
	}
}

func TestMetrics_UnmatchedRouteLabel(t *testing.T) {
	obs := &fakeObserver{}
	r := chi.NewRouter()
	r.Use(Metrics(obs))
	r.Get("/known", func(w http.ResponseWriter, req *http.Request) { w.WriteHeader(http.StatusOK) })

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/nope", nil))

	if len(obs.observed) != 1 {
		t.Fatalf("expected one observation, got %d", len(obs.observed))
	}
	if obs.observed[0].route != "unmatched" {
		t.Fatalf("unexpected route label: %q", obs.observed[0].route)
	}
	if obs.observed[0].status != http.StatusNotFound {
		t.Fatalf("unexpected status: %d", obs.observed[0].status)
	}
}

func TestRoutePattern(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	if got := RoutePattern(req); got != "unmatched" {
		t.Fatalf("got %q", got)
	}
}
