//go:build vim4

package router

import (
	"NanoKVM-Server/common"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLegacyScreenAndModeShareCodecAndPreserveCaptureResolution(t *testing.T) {
	vision := common.GetKvmVision()
	previous := vision.StreamType
	defer vision.SetStreamType(previous)
	settings := vision.VideoSettings()
	r := gin.New()
	r.POST("/screen", setScreenVIM4)
	r.POST("/mode", setModeVIM4)
	r.POST("/bitrate", setBitrateVIM4)
	request := func(path, body string) string {
		req := httptest.NewRequest("POST", path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rsp := httptest.NewRecorder()
		r.ServeHTTP(rsp, req)
		return rsp.Body.String()
	}
	if body := request("/screen", `{"type":"type","value":2}`); !strings.Contains(body, `"code":0`) || vision.StreamType != common.STREAM_TYPE_H265_WEBRTC {
		t.Fatalf("H265: %s", body)
	}
	if body := request("/mode", `{"mode":"direct"}`); !strings.Contains(body, `"code":0`) || vision.StreamType != common.STREAM_TYPE_H264_WEBRTC {
		t.Fatalf("direct: %s", body)
	}
	if body := request("/screen", `{"type":"resolution","value":600}`); !strings.Contains(body, `"code":0`) || vision.VideoSettings() != settings {
		t.Fatalf("display changed capture: %s", body)
	}
	for _, body := range []string{`{"type":"type","value":3}`, `{"type":"gop","value":-1}`, `{"type":"quality","value":99999}`, `{"type":"fps","value":61}`} {
		if response := request("/screen", body); strings.Contains(response, `"code":0`) {
			t.Errorf("invalid screen setting accepted: %s", body)
		}
	}
	if response := request("/bitrate", `{"bitrate":0}`); strings.Contains(response, `"code":0`) {
		t.Fatal("invalid bitrate accepted")
	}
	if vision.VideoSettings() != settings {
		t.Fatal("invalid requests changed capture settings")
	}
}
