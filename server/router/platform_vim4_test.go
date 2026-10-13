//go:build vim4

package router

import (
	"NanoKVM-Server/authn"
	"NanoKVM-Server/config"
	"NanoKVM-Server/middleware"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestUbuntuRoutesPreserveAuthenticationAndAdminBoundary(t *testing.T) {
	gin.SetMode(gin.TestMode)
	conf := config.GetInstance()
	previous := conf.Authentication
	conf.Authentication = "enable"
	defer func() { conf.Authentication = previous }()
	oldStore := authn.DefaultStore
	store := authn.NewStore(filepath.Join(t.TempDir(), "pwd"))
	authn.DefaultStore = store
	defer func() { authn.DefaultStore = oldStore }()
	if err := store.Create("viewer", "viewer-password", authn.RoleUser); err != nil {
		t.Fatal(err)
	}
	viewer, err := store.Get("viewer")
	if err != nil {
		t.Fatal(err)
	}
	token, err := middleware.GenerateJWT(viewer.Username, viewer.TokenVersion)
	if err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	vmRouter(r)
	networkRouter(r)
	extensionsRouter(r)
	platformRouter(r)
	for _, path := range []string{"/api/network/dns", "/api/network/static-ip", "/api/vm/timezone", "/api/vm/device/virtual", "/api/extensions/tailscale/start", "/api/extensions/assistant/start", "/api/vm/edid"} {
		for _, authenticated := range []bool{false, true} {
			req := httptest.NewRequest("POST", path, strings.NewReader(`{}`))
			req.Header.Set("Content-Type", "application/json")
			if authenticated {
				req.AddCookie(&http.Cookie{Name: middleware.CookieName, Value: token})
			}
			rsp := httptest.NewRecorder()
			r.ServeHTTP(rsp, req)
			want := http.StatusUnauthorized
			if authenticated {
				want = http.StatusForbidden
			}
			if rsp.Code != want {
				t.Errorf("%s authenticated=%v: got %d, want %d: %s", path, authenticated, rsp.Code, want, rsp.Body.String())
			}
		}
	}
}

func TestSnapshotRejectsUnauthenticatedRemoteAndLoopbackRequests(t *testing.T) {
	conf := config.GetInstance()
	previous := conf.Authentication
	conf.Authentication = "enable"
	defer func() { conf.Authentication = previous }()
	r := gin.New()
	streamRouterVIM4(r)
	for _, remote := range []string{"192.0.2.10:5000", "127.0.0.1:5000"} {
		for _, token := range []string{"", "invalid-internal-token"} {
			req := httptest.NewRequest("GET", "/api/stream/snapshot", nil)
			req.RemoteAddr = remote
			req.Header.Set(config.PicoclawInternalTokenHeader, token)
			rsp := httptest.NewRecorder()
			r.ServeHTTP(rsp, req)
			if rsp.Code != http.StatusUnauthorized {
				t.Fatalf("snapshot from %s without valid credentials: %d", remote, rsp.Code)
			}
		}
	}
}

func TestUbuntuInformationAndUnconfiguredHardwareNeedNoApplianceCommands(t *testing.T) {
	conf := config.GetInstance()
	previous := conf.Authentication
	conf.Authentication = "disable"
	defer func() { conf.Authentication = previous }()
	t.Setenv("PATH", t.TempDir())
	r := gin.New()
	vmRouter(r)
	hidRouter(r)
	for _, path := range []string{"/api/vm/info", "/api/vm/hardware", "/api/vm/swap", "/api/vm/gpio", "/api/vm/oled", "/api/vm/device/virtual", "/api/hid/mode"} {
		rsp := httptest.NewRecorder()
		r.ServeHTTP(rsp, httptest.NewRequest("GET", path, nil))
		var result struct {
			Code int            `json:"code"`
			Data map[string]any `json:"data"`
		}
		if err := json.Unmarshal(rsp.Body.Bytes(), &result); err != nil || result.Code != 0 {
			t.Errorf("%s: %s (%v)", path, rsp.Body.String(), err)
		}
		if path == "/api/vm/device/virtual" {
			for _, key := range []string{"disk", "network", "media", "isMicEnabled", "isNetworkEnabled"} {
				if _, ok := result.Data[key].(bool); !ok {
					t.Errorf("missing %s", key)
				}
			}
		}
	}
	for _, path := range []string{"/api/vm/gpio", "/api/vm/swap", "/api/vm/hdmi/reset", "/api/vm/hdmi/enable", "/api/vm/hdmi/disable", "/api/vm/hdmi/timeout", "/api/hid/mode"} {
		rsp := httptest.NewRecorder()
		req := httptest.NewRequest("POST", path, strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(rsp, req)
		if !strings.Contains(rsp.Body.String(), "not configured on VIM4") {
			t.Errorf("%s: %s", path, rsp.Body.String())
		}
	}
}
