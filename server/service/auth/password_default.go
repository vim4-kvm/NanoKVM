//go:build !vim4

package auth

func platformPasswordUpdater(password string) error { return changeRootPassword(password) }
