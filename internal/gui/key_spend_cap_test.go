package gui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yetone/magpie/internal/fx"
	"github.com/yetone/magpie/internal/provider"
)

// A key's spend cap through the GUI's own routes: provider/keyspendcap
// takes an amount in USD or in yuan, a cap in yuan worked out in USD at the
// rate magpie last learned and both kept in /api/providers' keyList, and
// none takes it off.
func TestKeySpendCapRoutesInYuan(t *testing.T) {
	h := t.TempDir()
	t.Setenv("HOME", h)
	t.Setenv("USERPROFILE", h)
	t.Setenv("APPDATA", filepath.Join(h, "AppData"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(h, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(h, ".cache"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(h, ".data"))
	if err := os.MkdirAll(filepath.Dir(fx.CachePath()), 0o755); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(fx.Rate{CNYPerUSD: 7.2, At: time.Now()})
	if err := os.WriteFile(fx.CachePath(), b, 0o644); err != nil {
		t.Fatal(err)
	}
	fx.Reset()
	t.Cleanup(fx.Reset)
	mux := http.NewServeMux()
	providerRoutes(mux, nil)
	call := func(method, path, body string) string {
		t.Helper()
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest(method, path, strings.NewReader(body)))
		if w.Code != 200 {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body)
		}
		return w.Body.String()
	}
	key := provider.KeyID("sk-a")
	listed := func() *provider.KeySpend {
		t.Helper()
		var out struct {
			Providers []struct {
				ID      string `json:"id"`
				KeyList []struct {
					ID    string             `json:"id"`
					Spend *provider.KeySpend `json:"spend"`
				} `json:"keyList"`
			} `json:"providers"`
		}
		if err := json.Unmarshal([]byte(call("GET", "/api/providers", "")), &out); err != nil {
			t.Fatal(err)
		}
		for _, p := range out.Providers {
			if p.ID != "relay" {
				continue
			}
			for _, k := range p.KeyList {
				if k.ID == key {
					return k.Spend
				}
			}
		}
		t.Fatal("relay's key not listed")
		return nil
	}
	call("POST", "/api/provider/save", `{"id":"relay","name":"Relay","chat":"http://127.0.0.1:1/v1","key":"sk-a","new":true}`)
	call("POST", "/api/provider/keyspendcap", `{"id":"relay","account":"`+key+`","spend":{"cny":720,"period":"month"}}`)
	sp := listed()
	if sp == nil || sp.CNY == nil || *sp.CNY != 720 || sp.USD != 100 || sp.Rate != 7.2 || sp.Period != provider.SpendMonth {
		t.Fatalf("a cap in yuan: %+v", sp)
	}
	// in USD as before, and none takes it off
	call("POST", "/api/provider/keyspendcap", `{"id":"relay","account":"`+key+`","spend":{"usd":12.5,"period":"day"}}`)
	sp = listed()
	if sp == nil || sp.CNY != nil || sp.USD != 12.5 || sp.Rate != 0 || sp.Period != provider.SpendDay {
		t.Fatalf("a cap in USD: %+v", sp)
	}
	for _, body := range []string{
		`{"id":"relay","account":"` + key + `","spend":{"usd":5,"cny":36,"period":"day"}}`,
		`{"id":"relay","account":"` + key + `","spend":{"cny":0,"period":"day"}}`,
		`{"id":"relay","account":"` + key + `","spend":{"cny":5,"period":"week"}}`,
	} {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest("POST", "/api/provider/keyspendcap", strings.NewReader(body)))
		if w.Code == 200 {
			t.Errorf("%s taken", body)
		}
	}
	if sp = listed(); sp == nil || sp.USD != 12.5 || sp.Period != provider.SpendDay {
		t.Fatalf("a refused change changed the cap: %+v", sp)
	}
	call("POST", "/api/provider/keyspendcap", `{"id":"relay","account":"`+key+`","spend":null}`)
	if sp = listed(); sp != nil {
		t.Fatalf("taken off: %+v", sp)
	}
}
