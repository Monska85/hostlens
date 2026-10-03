package auth

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStoreCredentialLifecycle(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	store := Store{Path: filepath.Join(dir, "tokens.json")}
	record, secret, err := store.Create(context.Background(), "operator", []string{"observe", "repair"}, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if secret == "" || record.Hash != "" {
		t.Fatal("secret or hash output invalid")
	}
	verified, err := store.Verify(secret)
	if err != nil || verified.ID != record.ID || verified.Hash != "" {
		t.Fatalf("valid credential rejected or hash exposed: %v", err)
	}
	if _, err := store.Verify(secret + "x"); err == nil {
		t.Fatal("invalid credential accepted")
	}
	listed, err := store.List()
	if err != nil || len(listed) != 1 || listed[0].Hash != "" {
		t.Fatalf("token listing invalid: %v", err)
	}
	if err := store.Revoke(context.Background(), record.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Verify(secret); err == nil {
		t.Fatal("revoked credential accepted")
	}
}

func TestStoreRejectsUnsafeState(t *testing.T) {
	dir := t.TempDir()
	store := Store{Path: filepath.Join(dir, "tokens.json")}
	if err := os.Chmod(dir, 0777); err != nil {
		t.Fatal(err)
	}
	if _, err := store.List(); err == nil {
		t.Fatal("writable directory accepted")
	}
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.Path, []byte("[] {}"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.List(); err == nil {
		t.Fatal("trailing document accepted")
	}
}

func TestRotatePublishesNewSecretAndRetiresOldAtDeadline(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "tokens.json")}
	old, oldSecret, err := store.Create(context.Background(), "client", []string{"observe"}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
	if _, _, err := store.Rotate(context.Background(), old.ID, time.Now().Add(30*time.Minute), deadline); err == nil {
		t.Fatal("overlap beyond new expiry accepted")
	}
	current, currentSecret, err := store.Rotate(context.Background(), old.ID, time.Time{}, deadline)
	if err != nil {
		t.Fatal(err)
	}
	if current.ID == old.ID || currentSecret == oldSecret || len(current.Roles) != 1 || current.Roles[0] != "observe" {
		t.Fatal("rotation identity or role mismatch")
	}
	if _, err := store.Verify(currentSecret); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Verify(oldSecret); err != nil {
		t.Fatal("old secret expired before overlap")
	}
	records, err := store.List()
	if err != nil || len(records) != 2 || !records[0].Expires.Equal(deadline) {
		t.Fatalf("old token retirement was not recorded: %+v, %v", records, err)
	}
}
