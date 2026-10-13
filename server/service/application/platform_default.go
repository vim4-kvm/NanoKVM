//go:build !vim4

package application

import "github.com/gin-gonic/gin"

const (
	StableURL          = "https://cdn.sipeed.com/nanokvm"
	PreviewURL         = "https://cdn.sipeed.com/nanokvm/preview"
	AppDir             = "/kvmapp"
	BackupDir          = "/root/old"
	CacheDir           = "/root/.kvmcache"
	latestManifestName = "latest.json"
	packagePattern     = `^nanokvm_[0-9]+\.[0-9]+\.[0-9]+\.tar\.gz$`
)

func platformUpdate(*gin.Context, bool) bool { return false }
