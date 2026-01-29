package cache

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStore_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	store := New(dir, DefaultTTL)

	type payload struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}

	original := payload{Name: "test", Count: 42}
	if err := store.Save("mykey", original); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	var loaded payload
	found, err := store.Load("mykey", &loaded)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if !found {
		t.Fatal("expected cache hit but got miss")
	}
	if loaded.Name != original.Name || loaded.Count != original.Count {
		t.Errorf("loaded payload mismatch: got %+v, want %+v", loaded, original)
	}
}

func TestStore_LoadMiss(t *testing.T) {
	dir := t.TempDir()
	store := New(dir, DefaultTTL)

	var out string
	found, err := store.Load("nonexistent", &out)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if found {
		t.Error("expected cache miss but got hit")
	}
}

func TestStore_Expiry(t *testing.T) {
	dir := t.TempDir()
	store := New(dir, 1*time.Second)

	// Set clock to the past
	pastTime := time.Now().Add(-2 * time.Second)
	store.Clock = func() time.Time { return pastTime }

	if err := store.Save("expiring", "value"); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Reset clock to now so TTL has passed
	store.Clock = time.Now

	var out string
	found, err := store.Load("expiring", &out)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if found {
		t.Error("expected cache miss due to expiry but got hit")
	}
}

func TestStore_Expire(t *testing.T) {
	dir := t.TempDir()
	store := New(dir, DefaultTTL)

	if err := store.Save("toremove", "data"); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if err := store.Expire("toremove"); err != nil {
		t.Fatalf("Expire failed: %v", err)
	}

	var out string
	found, _ := store.Load("toremove", &out)
	if found {
		t.Error("expected cache miss after Expire but got hit")
	}
}

func TestStore_ExpireAll(t *testing.T) {
	dir := t.TempDir()
	store := New(dir, DefaultTTL)

	_ = store.Save("channels_all", "ch1")
	_ = store.Save("channels_extra", "ch2")
	_ = store.Save("users_all", "u1")

	if err := store.ExpireAll("channels"); err != nil {
		t.Fatalf("ExpireAll failed: %v", err)
	}

	var out string
	if found, _ := store.Load("channels_all", &out); found {
		t.Error("channels_all should be expired")
	}
	if found, _ := store.Load("channels_extra", &out); found {
		t.Error("channels_extra should be expired")
	}
	if found, _ := store.Load("users_all", &out); !found {
		t.Error("users_all should still exist")
	}
}

func TestStore_CorruptedEntry(t *testing.T) {
	dir := t.TempDir()
	store := New(dir, DefaultTTL)

	// Write garbage directly
	path := filepath.Join(dir, "corrupt.json")
	if err := os.WriteFile(path, []byte("not valid json"), 0o600); err != nil {
		t.Fatal(err)
	}

	var out string
	found, err := store.Load("corrupt", &out)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if found {
		t.Error("expected cache miss for corrupted entry")
	}
	// File should be removed
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("corrupted file should have been removed")
	}
}

func TestStore_AtomicWrite(t *testing.T) {
	dir := t.TempDir()
	store := New(dir, DefaultTTL)

	if err := store.Save("atomic", "value"); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Ensure no .tmp file remains
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp" {
			t.Errorf("temp file should not remain: %s", e.Name())
		}
	}
}
