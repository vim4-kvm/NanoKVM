//go:build !vim4

package common

func checkPlatformScreen(*Screen) bool { return false }
