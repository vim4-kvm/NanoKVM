//go:build linux && vim4

package common

func checkPlatformScreen(s *Screen) bool {
	// The capture pipeline owns the saved FPS limit. Polling at the appliance's
	// default 30 Hz would otherwise discard hardware JPEG frames above 30 fps.
	s.FPS = 60
	s.Width, s.Height = GetKvmVision().Dimensions()
	if s.Width == 0 || s.Height == 0 {
		settings := GetKvmVision().VideoSettings()
		s.Width, s.Height = uint16(settings.Width), uint16(settings.Height)
	}
	return true
}
