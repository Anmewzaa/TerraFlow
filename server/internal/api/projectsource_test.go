package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOnlyForAllowsTheConfiguredProjectSourceAndRefusesTheOther(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	for _, c := range []struct {
		route, mode string
		want        int
	}{
		{"github", "github", http.StatusNoContent},
		{"local", "local", http.StatusNoContent},
		{"github", "local", http.StatusConflict},
		{"local", "github", http.StatusConflict},
	} {
		w := httptest.NewRecorder()
		onlyFor(c.route, c.mode, ok)(w, httptest.NewRequest(http.MethodGet, "/", nil))
		if w.Code != c.want {
			t.Errorf("route %s in mode %s: got %d, want %d", c.route, c.mode, w.Code, c.want)
		}
	}
}
