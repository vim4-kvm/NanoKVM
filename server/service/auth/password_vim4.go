//go:build vim4

package auth

// Web accounts never change Ubuntu login credentials.
func platformPasswordUpdater(string) error { return nil }
