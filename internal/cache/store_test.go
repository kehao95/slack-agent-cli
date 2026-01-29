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

func TestStore_PartialCache(t *testing.T) {
	dir := t.TempDir()
	store := New(dir, DefaultTTL)

	type Item struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}

	// Save partial data with a cursor
	items := []Item{
		{ID: "1", Name: "first"},
		{ID: "2", Name: "second"},
	}
	if err := store.SavePartial("items", items, "cursor123", false, 2); err != nil {
		t.Fatalf("SavePartial failed: %v", err)
	}

	// Load partial and verify state
	var loaded []Item
	state, found, err := store.LoadPartial("items", &loaded)
	if err != nil {
		t.Fatalf("LoadPartial failed: %v", err)
	}
	if !found {
		t.Fatal("expected partial cache hit")
	}
	if state.Complete {
		t.Error("expected Complete=false")
	}
	if state.NextCursor != "cursor123" {
		t.Errorf("expected cursor123, got %s", state.NextCursor)
	}
	if state.Count != 2 {
		t.Errorf("expected count 2, got %d", state.Count)
	}
	if len(loaded) != 2 {
		t.Errorf("expected 2 items, got %d", len(loaded))
	}
}

func TestStore_PartialCacheResume(t *testing.T) {
	dir := t.TempDir()
	store := New(dir, DefaultTTL)

	type Item struct {
		ID string `json:"id"`
	}

	// First page
	page1 := []Item{{ID: "1"}, {ID: "2"}}
	if err := store.SavePartial("items", page1, "cursor_page2", false, 2); err != nil {
		t.Fatalf("SavePartial page1 failed: %v", err)
	}

	// Load and verify cursor for resume
	var loaded []Item
	state, found, err := store.LoadPartial("items", &loaded)
	if err != nil || !found {
		t.Fatalf("LoadPartial failed: err=%v, found=%v", err, found)
	}
	if state.NextCursor != "cursor_page2" {
		t.Errorf("expected cursor_page2, got %s", state.NextCursor)
	}

	// Second page (complete)
	page2 := []Item{{ID: "1"}, {ID: "2"}, {ID: "3"}, {ID: "4"}}
	if err := store.SavePartial("items", page2, "", true, 4); err != nil {
		t.Fatalf("SavePartial page2 failed: %v", err)
	}

	state, found, err = store.LoadPartial("items", &loaded)
	if err != nil || !found {
		t.Fatalf("LoadPartial failed: err=%v, found=%v", err, found)
	}
	if !state.Complete {
		t.Error("expected Complete=true after final page")
	}
	if state.Count != 4 {
		t.Errorf("expected count 4, got %d", state.Count)
	}
}

func TestStore_PartialCacheExpiry(t *testing.T) {
	dir := t.TempDir()
	store := New(dir, DefaultTTL)

	// Set clock to the past (beyond PartialTTL of 1 day)
	pastTime := time.Now().Add(-25 * time.Hour)
	store.Clock = func() time.Time { return pastTime }

	if err := store.SavePartial("items", []string{"a"}, "cursor", false, 1); err != nil {
		t.Fatalf("SavePartial failed: %v", err)
	}

	// Reset clock to now
	store.Clock = time.Now

	var out []string
	_, found, err := store.LoadPartial("items", &out)
	if err != nil {
		t.Fatalf("LoadPartial error: %v", err)
	}
	if found {
		t.Error("expected partial cache miss due to expiry")
	}
}

func TestStore_PromotePartial(t *testing.T) {
	dir := t.TempDir()
	store := New(dir, DefaultTTL)

	// Save as partial first
	items := []string{"a", "b", "c"}
	if err := store.SavePartial("items", items, "", true, 3); err != nil {
		t.Fatalf("SavePartial failed: %v", err)
	}

	// Promote to main cache
	if err := store.PromotePartial("items", items); err != nil {
		t.Fatalf("PromotePartial failed: %v", err)
	}

	// Should be loadable from main cache
	var loaded []string
	found, err := store.Load("items", &loaded)
	if err != nil || !found {
		t.Fatalf("Load failed: err=%v, found=%v", err, found)
	}
	if len(loaded) != 3 {
		t.Errorf("expected 3 items, got %d", len(loaded))
	}

	// Partial file should be gone
	var partialOut []string
	_, found, _ = store.LoadPartial("items", &partialOut)
	if found {
		t.Error("partial cache should be cleared after promotion")
	}
}

func TestStore_ExpirePartial(t *testing.T) {
	dir := t.TempDir()
	store := New(dir, DefaultTTL)

	if err := store.SavePartial("items", []string{"a"}, "cursor", false, 1); err != nil {
		t.Fatalf("SavePartial failed: %v", err)
	}

	if err := store.ExpirePartial("items"); err != nil {
		t.Fatalf("ExpirePartial failed: %v", err)
	}

	var out []string
	_, found, _ := store.LoadPartial("items", &out)
	if found {
		t.Error("expected partial cache miss after ExpirePartial")
	}
}

func TestStore_GetStatus(t *testing.T) {
	dir := t.TempDir()
	store := New(dir, DefaultTTL)

	// Save main cache
	channels := []string{"ch1", "ch2", "ch3"}
	if err := store.Save(CacheKeyChannels, channels); err != nil {
		t.Fatalf("Save channels failed: %v", err)
	}

	// Save partial cache
	users := []string{"u1", "u2"}
	if err := store.SavePartial(CacheKeyUsers, users, "cursor_next", false, 2); err != nil {
		t.Fatalf("SavePartial users failed: %v", err)
	}

	// Check channels status
	chStatus, ok := store.GetStatus(CacheKeyChannels)
	if !ok {
		t.Fatal("channels status missing")
	}
	if chStatus.Count != 3 {
		t.Errorf("expected channels count 3, got %d", chStatus.Count)
	}
	if !chStatus.Complete {
		t.Error("expected channels Complete=true")
	}

	// Check users status (partial)
	uStatus, ok := store.GetStatus(CacheKeyUsers)
	if !ok {
		t.Fatal("users status missing")
	}
	if uStatus.Count != 2 {
		t.Errorf("expected users count 2, got %d", uStatus.Count)
	}
	if uStatus.Complete {
		t.Error("expected users Complete=false (partial)")
	}
	if uStatus.NextCursor != "cursor_next" {
		t.Errorf("expected cursor_next, got %s", uStatus.NextCursor)
	}
}
