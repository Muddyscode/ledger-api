package integration

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Muddyscode/ledger-api/internal/config"
	"github.com/Muddyscode/ledger-api/internal/console"
	"github.com/Muddyscode/ledger-api/internal/store"
	"github.com/go-chi/chi/v5"
)

func TestConsoleLoginRenders(t *testing.T) {
	r := chi.NewRouter()
	cfg := config.Config{
		JWTSecret: "test-secret-must-be-32-bytes-ok!",
		SeedEmail: "operator@ledger.local",
	}
	if err := console.Mount(r, cfg, store.New(testPool)); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	res, err := http.Get(srv.URL + "/login")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != 200 {
		t.Fatalf("login page %d", res.StatusCode)
	}
	if !strings.Contains(string(body), "Operator console") {
		t.Fatalf("unexpected login html: %s", body)
	}
}
