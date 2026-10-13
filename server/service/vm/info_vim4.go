//go:build vim4

package vm

import (
	"NanoKVM-Server/proto"
	"github.com/gin-gonic/gin"
	"os"
	"strings"
)

func (s *Service) GetUbuntuInfo(c *gin.Context) {
	version, _ := os.ReadFile("/opt/vim4-nanokvm/version")
	hostname, _ := os.Hostname()
	distribution := "Ubuntu"
	if data, err := os.ReadFile("/etc/os-release"); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if value, ok := strings.CutPrefix(line, "PRETTY_NAME="); ok {
				distribution = strings.Trim(value, "\"")
			}
		}
	}
	ips := getIPs()
	if ips == nil {
		ips = []proto.IP{}
	}
	var rsp proto.Response
	rsp.OkRspWithData(c, proto.GetInfoRsp{IPs: ips, Mdns: hostname + ".local", Image: distribution, Application: strings.TrimSpace(string(version)), DeviceKey: ""})
}
