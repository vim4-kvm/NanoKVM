//go:build !vim4

package router

import "github.com/gin-gonic/gin"

func platformHandler(_ string, handler gin.HandlerFunc) gin.HandlerFunc { return handler }
func platformRouter(*gin.Engine)                                        {}

func platformLoopbackHTTPPaths() []string { return nil }
