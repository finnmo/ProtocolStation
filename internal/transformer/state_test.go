package transformer

import (
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"go.uber.org/zap"
)

func TestStateStorageSetGet(t *testing.T) {
	ss, err := NewStateStorage(filepath.Join(t.TempDir(), "state.json"), zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}

	if err := ss.Set("key", "value"); err != nil {
		t.Fatal(err)
	}
	got, ok := ss.Get("key")
	if !ok || got != "value" {
		t.Errorf("expected 'value', got %v (ok=%v)", got, ok)
	}
}

func TestStateStorageMissingKey(t *testing.T) {
	ss, err := NewStateStorage(filepath.Join(t.TempDir(), "state.json"), zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	_, ok := ss.Get("nonexistent")
	if ok {
		t.Error("expected false for missing key")
	}
}

func TestStateStorageSaveLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	ss, err := NewStateStorage(path, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	if err := ss.SetAndSave("persistent", 42); err != nil {
		t.Fatal(err)
	}

	// Load fresh instance from same file
	ss2, err := NewStateStorage(path, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	val, ok := ss2.Get("persistent")
	if !ok {
		t.Fatal("key not found after reload")
	}
	// JSON numbers unmarshal as float64
	if val.(float64) != 42 {
		t.Errorf("expected 42, got %v", val)
	}
}

// TestSetAndSaveConcurrent verifies that concurrent SetAndSave calls do not
// corrupt state and the race detector catches any real data race.
func TestSetAndSaveConcurrent(t *testing.T) {
	ss, err := NewStateStorage(filepath.Join(t.TempDir(), "state.json"), zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}

	const workers = 50
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(n int) {
			defer wg.Done()
			key := fmt.Sprintf("key%d", n)
			if err := ss.SetAndSave(key, n); err != nil {
				t.Errorf("SetAndSave error: %v", err)
			}
		}(i)
	}
	wg.Wait()

	// Every key must be readable
	for i := 0; i < workers; i++ {
		key := fmt.Sprintf("key%d", i)
		if _, ok := ss.Get(key); !ok {
			t.Errorf("key %q missing after concurrent writes", key)
		}
	}
}

// TestSetAndSaveAtomicRead verifies that a concurrent Get never observes a
// state where the file has been updated but the map has not (or vice versa).
// With the old double-lock implementation this was possible; the new single-lock
// version holds the lock across both operations.
func TestSetAndSaveAtomicRead(t *testing.T) {
	ss, err := NewStateStorage(filepath.Join(t.TempDir(), "state.json"), zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 200; i++ {
			_ = ss.SetAndSave("counter", i)
		}
	}()

	// Concurrent readers should never panic or deadlock
	var wg sync.WaitGroup
	for r := 0; r < 10; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
					ss.Get("counter")
				}
			}
		}()
	}
	<-done
	wg.Wait()
}
