//go:build vim4

package download

import "NanoKVM-Server/service/storage"

var imageDirectory = storage.ImageDirectory

func publishImage(temp, target string) error { return storage.PublishImage(temp, target) }
