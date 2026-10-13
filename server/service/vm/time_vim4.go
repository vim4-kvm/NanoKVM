//go:build vim4

package vm

import (
	"NanoKVM-Server/proto"
	"NanoKVM-Server/utils"
	"github.com/gin-gonic/gin"
	"io"
	"os"
	"strings"
	"time"
)

func (s *Service) GetTimeZone(c *gin.Context) {
	var rsp proto.Response
	zone, _ := os.Readlink("/etc/localtime")
	if i := strings.Index(zone, "zoneinfo/"); i >= 0 {
		zone = zone[i+len("zoneinfo/"):]
	} else {
		data, _ := os.ReadFile("/etc/timezone")
		zone = strings.TrimSpace(string(data))
	}
	if _, err := time.LoadLocation(zone); err != nil || zone == "" {
		rsp.ErrRsp(c, -1, "cannot read timezone")
		return
	}
	rsp.OkRspWithData(c, gin.H{"timezone": zone})
}
func (s *Service) GetTime(c *gin.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	c.Stream(func(io.Writer) bool {
		select {
		case <-c.Request.Context().Done():
			return false
		case now := <-ticker.C:
			c.SSEvent("message", now.UnixMilli())
			return true
		}
	})
}
func (s *Service) GetTimeStatus(c *gin.Context) {
	var rsp proto.Response
	status, err := utils.GetChronySyncStatus()
	if err != nil {
		rsp.ErrRsp(c, -1, "cannot read clock synchronization")
		return
	}
	rsp.OkRspWithData(c, gin.H{"isSynchronized": status.IsSynchronized, "lastSyncTime": status.LastSyncTime})
}
