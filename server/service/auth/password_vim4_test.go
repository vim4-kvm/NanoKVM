//go:build vim4

package auth

import (
	"NanoKVM-Server/authn"
	"encoding/json"
	"golang.org/x/crypto/bcrypt"
	"os"
	"path/filepath"
	"testing"
)

func TestLegacyOwnerPasswordMigrationDoesNotInvokeUbuntuPasswd(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pwd")
	data, _ := json.Marshal(map[string]string{"username": "owner", "password": encryptForRequest("old-password")})
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	store := authn.NewStore(path)
	restore := useTestStore(store)
	defer restore()
	t.Setenv("PATH", t.TempDir())
	owner, ok, err := store.Authenticate("owner", "old-password")
	if err != nil || !ok {
		t.Fatalf("migration: %v %v", ok, err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(owner.PasswordHash), []byte("old-password")); err != nil {
		t.Fatal(err)
	}
	if err := changeUserPassword("owner", encryptForRequest("new-password")); err != nil {
		t.Fatal(err)
	}
	_, ok, err = store.Authenticate("owner", "new-password")
	if err != nil || !ok {
		t.Fatalf("new password: %v %v", ok, err)
	}
	if _, err := store.ValidateToken("owner", owner.TokenVersion); err == nil {
		t.Fatal("old token survived password change")
	}
}
