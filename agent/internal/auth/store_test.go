package auth

import (
	"path/filepath"
	"testing"
)

func TestAddAuthenticateAndReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "users.json")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Add("admin", "admin", "correct horse battery staple"); err != nil {
		t.Fatal(err)
	}
	if _, ok := store.Authenticate("admin", "wrong password value"); ok {
		t.Fatal("unexpected authentication success")
	}
	user, ok := store.Authenticate("ADMIN", "correct horse battery staple")
	if !ok || user.Role != "admin" {
		t.Fatalf("authentication failed: %#v", user)
	}
	reloaded, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := reloaded.Authenticate("admin", "correct horse battery staple"); !ok {
		t.Fatal("reloaded store could not authenticate")
	}
}

func TestRejectsWeakInputsAndDuplicates(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "users.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Add("x", "admin", "correct horse battery staple"); err == nil {
		t.Fatal("short username was accepted")
	}
	if err := store.Add("operator", "owner", "correct horse battery staple"); err == nil {
		t.Fatal("unknown role was accepted")
	}
	if err := store.Add("operator", "operator", "too-short"); err == nil {
		t.Fatal("short password was accepted")
	}
	if err := store.Add("operator", "operator", "correct horse battery staple"); err != nil {
		t.Fatal(err)
	}
	if err := store.Add("OPERATOR", "operator", "another correct battery staple"); err == nil {
		t.Fatal("case-insensitive duplicate was accepted")
	}
}
