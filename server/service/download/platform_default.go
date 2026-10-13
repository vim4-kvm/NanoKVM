//go:build !vim4

package download

import "os"

const imageDirectory = "/data"

func publishImage(temp, target string) error { return os.Rename(temp, target) }
