package main

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// setupArenaCacheTest initializes the arena cache with a temp directory
// and returns a cleanup function that restores original state.
func setupArenaCacheTest(t *testing.T) func() {
	t.Helper()

	origDir := arenaOHLCVCacheDir
	origCache := arenaOHLCVMemCache

	tmpDir := t.TempDir()
	arenaOHLCVCacheDir = tmpDir

	arenaOHLCVMemCacheMu.Lock()
	arenaOHLCVMemCache = make(map[string]map[string]*ohlcvCacheEntry)
	arenaOHLCVMemCacheMu.Unlock()

	// Also init the shared cache to ensure isolation tests work
	ohlcvMemCacheMu.Lock()
	if ohlcvMemCache == nil {
		ohlcvMemCache = make(map[string]map[string]*ohlcvCacheEntry)
	}
	ohlcvMemCacheMu.Unlock()

	return func() {
		time.Sleep(100 * time.Millisecond) // wait for async file writes to finish
		arenaOHLCVCacheDir = origDir
		arenaOHLCVMemCacheMu.Lock()
		arenaOHLCVMemCache = origCache
		arenaOHLCVMemCacheMu.Unlock()
	}
}

// TestArenaCache_SetAndGet verifies basic write→read cycle in arena memory cache
func TestArenaCache_SetAndGet(t *testing.T) {
	cleanup := setupArenaCacheTest(t)
	defer cleanup()

	bars := []OHLCV{
		{Time: 1000, Open: 100, High: 110, Low: 90, Close: 105, Volume: 1000},
		{Time: 2000, Open: 105, High: 115, Low: 95, Close: 110, Volume: 1200},
		{Time: 3000, Open: 110, High: 120, Low: 100, Close: 115, Volume: 800},
	}

	setArenaOHLCVInMemCache("AAPL", "60m", bars)

	got, ok := getArenaOHLCVFromMemCache("AAPL", "60m")
	if !ok {
		t.Fatal("expected to find AAPL in arena cache")
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 bars, got %d", len(got))
	}
	if got[0].Close != 105 {
		t.Errorf("expected Close=105, got %f", got[0].Close)
	}
}

// TestArenaCache_MissReturnsEmpty verifies that a cache miss returns false
func TestArenaCache_MissReturnsEmpty(t *testing.T) {
	cleanup := setupArenaCacheTest(t)
	defer cleanup()

	_, ok := getArenaOHLCVFromMemCache("UNKNOWN", "60m")
	if ok {
		t.Error("expected cache miss for unknown symbol")
	}
}

// TestArenaCache_IsolationFromLiveCache verifies that arena and live caches are independent.
// Writing to arena cache must NOT appear in live cache and vice versa.
func TestArenaCache_IsolationFromLiveCache(t *testing.T) {
	cleanup := setupArenaCacheTest(t)
	defer cleanup()

	// Also set up a temp dir for the live cache
	origLiveDir := ohlcvCacheDir
	ohlcvCacheDir = t.TempDir()
	defer func() { ohlcvCacheDir = origLiveDir }()

	arenaBars := []OHLCV{
		{Time: 1000, Open: 100, High: 110, Low: 90, Close: 105, Volume: 500},
	}
	liveBars := []OHLCV{
		{Time: 2000, Open: 200, High: 220, Low: 190, Close: 210, Volume: 999},
	}

	// Write to arena cache
	setArenaOHLCVInMemCache("TEST", "60m", arenaBars)
	// Write to live cache
	setOHLCVInMemCache("TEST", "60m", liveBars)

	// Arena should have arena data
	arenaGot, ok := getArenaOHLCVFromMemCache("TEST", "60m")
	if !ok {
		t.Fatal("expected to find TEST in arena cache")
	}
	if arenaGot[0].Close != 105 {
		t.Errorf("arena cache: expected Close=105, got %f", arenaGot[0].Close)
	}

	// Live should have live data
	liveGot, ok := getOHLCVFromMemCache("TEST", "60m")
	if !ok {
		t.Fatal("expected to find TEST in live cache")
	}
	if liveGot[0].Close != 210 {
		t.Errorf("live cache: expected Close=210, got %f", liveGot[0].Close)
	}

	// Cross-check: arena symbol in a different interval should NOT be in live
	setArenaOHLCVInMemCache("ARENA_ONLY", "5m", arenaBars)
	_, ok = getOHLCVFromMemCache("ARENA_ONLY", "5m")
	if ok {
		t.Error("arena-only symbol should NOT appear in live cache")
	}
}

// TestArenaCache_FileWriteAndRead verifies that data persists to gzip files
// and can be read back via the arena file path
func TestArenaCache_FileWriteAndRead(t *testing.T) {
	cleanup := setupArenaCacheTest(t)
	defer cleanup()

	bars := []OHLCV{
		{Time: 1000, Open: 100, High: 110, Low: 90, Close: 105, Volume: 500},
		{Time: 2000, Open: 105, High: 115, Low: 95, Close: 110, Volume: 600},
	}

	err := writeArenaOHLCVFile("MSFT", "60m", bars)
	if err != nil {
		t.Fatalf("writeArenaOHLCVFile failed: %v", err)
	}

	// File should exist in arena dir, not in live dir
	arenaPath := arenaOHLCVFilePath("MSFT", "60m")
	if _, err := os.Stat(arenaPath); os.IsNotExist(err) {
		t.Fatal("expected arena file to exist")
	}

	readBars, modTime, err := readArenaOHLCVFile("MSFT", "60m")
	if err != nil {
		t.Fatalf("readArenaOHLCVFile failed: %v", err)
	}
	if len(readBars) != 2 {
		t.Fatalf("expected 2 bars, got %d", len(readBars))
	}
	if readBars[1].Close != 110 {
		t.Errorf("expected Close=110, got %f", readBars[1].Close)
	}
	if modTime.IsZero() {
		t.Error("expected non-zero mod time")
	}
}

// TestArenaCache_FilePathSeparation verifies that arena and live file paths
// point to different directories
func TestArenaCache_FilePathSeparation(t *testing.T) {
	cleanup := setupArenaCacheTest(t)
	defer cleanup()

	origLiveDir := ohlcvCacheDir
	ohlcvCacheDir = t.TempDir()
	defer func() { ohlcvCacheDir = origLiveDir }()

	arenaPath := arenaOHLCVFilePath("AAPL", "60m")
	livePath := ohlcvFilePath("AAPL", "60m")

	if arenaPath == livePath {
		t.Errorf("arena and live paths should differ:\n  arena: %s\n  live:  %s", arenaPath, livePath)
	}

	arenaDir := filepath.Dir(arenaPath)
	liveDir := filepath.Dir(livePath)
	if arenaDir == liveDir {
		t.Errorf("arena and live directories should differ: %s", arenaDir)
	}
}

// TestArenaCache_LazyLoadFromFile verifies that getArenaOHLCVFromMemCache
// lazy-loads from file when not in memory
func TestArenaCache_LazyLoadFromFile(t *testing.T) {
	cleanup := setupArenaCacheTest(t)
	defer cleanup()

	bars := []OHLCV{
		{Time: 1000, Open: 100, High: 110, Low: 90, Close: 105, Volume: 500},
	}

	// Write directly to file (bypass memory)
	err := writeArenaOHLCVFile("LAZY", "60m", bars)
	if err != nil {
		t.Fatalf("writeArenaOHLCVFile failed: %v", err)
	}

	// Memory should be empty
	arenaOHLCVMemCacheMu.RLock()
	_, inMem := arenaOHLCVMemCache["LAZY"]
	arenaOHLCVMemCacheMu.RUnlock()
	if inMem {
		t.Fatal("expected LAZY not to be in memory yet")
	}

	// getArenaOHLCVFromMemCache should lazy-load from file
	got, ok := getArenaOHLCVFromMemCache("LAZY", "60m")
	if !ok {
		t.Fatal("expected lazy-load to succeed")
	}
	if len(got) != 1 || got[0].Close != 105 {
		t.Errorf("unexpected data after lazy-load: %v", got)
	}

	// Now it should be in memory
	arenaOHLCVMemCacheMu.RLock()
	_, inMem = arenaOHLCVMemCache["LAZY"]
	arenaOHLCVMemCacheMu.RUnlock()
	if !inMem {
		t.Error("expected LAZY to be in memory after lazy-load")
	}
}

// TestArenaCache_SaveOHLCVCacheWrapper verifies that saveArenaOHLCVCache
// stores data retrievable from memory
func TestArenaCache_SaveOHLCVCacheWrapper(t *testing.T) {
	cleanup := setupArenaCacheTest(t)
	defer cleanup()

	bars := []OHLCV{
		{Time: 1000, Open: 100, High: 110, Low: 90, Close: 105, Volume: 500},
	}

	saveArenaOHLCVCache("WRAP", "5m", bars)

	got, ok := getArenaOHLCVFromMemCache("WRAP", "5m")
	if !ok {
		t.Fatal("expected to find WRAP after saveArenaOHLCVCache")
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 bar, got %d", len(got))
	}
}

// TestArenaCache_GetSymbols verifies that getArenaOHLCVMemCacheSymbols returns
// correct symbols from both memory and disk
func TestArenaCache_GetSymbols(t *testing.T) {
	cleanup := setupArenaCacheTest(t)
	defer cleanup()

	// Put AAPL in memory
	setArenaOHLCVInMemCache("AAPL", "60m", []OHLCV{{Time: 1000, Close: 100}})

	// Put MSFT on disk only
	writeArenaOHLCVFile("MSFT", "60m", []OHLCV{{Time: 2000, Close: 200}})

	// Wait a tiny bit for async file write from setArenaOHLCVInMemCache
	time.Sleep(50 * time.Millisecond)

	symbols := getArenaOHLCVMemCacheSymbols("60m")
	if !symbols["AAPL"] {
		t.Error("expected AAPL in symbols")
	}
	if !symbols["MSFT"] {
		t.Error("expected MSFT in symbols")
	}

	// Different interval should not include these
	other := getArenaOHLCVMemCacheSymbols("5m")
	if other["AAPL"] {
		t.Error("AAPL should not be in 5m symbols")
	}
}

// TestArenaCache_ConcurrentAccess verifies thread safety of arena cache
func TestArenaCache_ConcurrentAccess(t *testing.T) {
	cleanup := setupArenaCacheTest(t)
	defer cleanup()

	var wg sync.WaitGroup
	symbols := []string{"AAPL", "MSFT", "GOOG", "TSLA", "AMZN", "META", "NVDA", "AMD"}

	// Concurrent writes
	for _, sym := range symbols {
		wg.Add(1)
		go func(s string) {
			defer wg.Done()
			bars := []OHLCV{{Time: 1000, Open: 100, High: 110, Low: 90, Close: 105, Volume: 500}}
			setArenaOHLCVInMemCache(s, "60m", bars)
		}(sym)
	}
	wg.Wait()

	// Concurrent reads
	for _, sym := range symbols {
		wg.Add(1)
		go func(s string) {
			defer wg.Done()
			got, ok := getArenaOHLCVFromMemCache(s, "60m")
			if !ok {
				t.Errorf("expected to find %s in arena cache", s)
			}
			if len(got) != 1 {
				t.Errorf("%s: expected 1 bar, got %d", s, len(got))
			}
		}(sym)
	}
	wg.Wait()

	// Verify all symbols appear in getArenaOHLCVMemCacheSymbols
	allSymbols := getArenaOHLCVMemCacheSymbols("60m")
	for _, sym := range symbols {
		if !allSymbols[sym] {
			t.Errorf("expected %s in arena cache symbols", sym)
		}
	}
}

// TestArenaCache_FilePathSanitization verifies that special characters
// in symbols are sanitized (e.g. BRK.B → BRK_B in filename)
func TestArenaCache_FilePathSanitization(t *testing.T) {
	cleanup := setupArenaCacheTest(t)
	defer cleanup()

	bars := []OHLCV{{Time: 1000, Close: 100}}

	err := writeArenaOHLCVFile("BRK.B", "60m", bars)
	if err != nil {
		t.Fatalf("writeArenaOHLCVFile failed: %v", err)
	}

	// Path should use underscore
	path := arenaOHLCVFilePath("BRK.B", "60m")
	expected := filepath.Join(arenaOHLCVCacheDir, "BRK.B_60m.json.gz")
	if path != expected {
		t.Errorf("expected path %s, got %s", expected, path)
	}

	// Read back should work
	readBars, _, err := readArenaOHLCVFile("BRK.B", "60m")
	if err != nil {
		t.Fatalf("readArenaOHLCVFile failed: %v", err)
	}
	if len(readBars) != 1 {
		t.Fatalf("expected 1 bar, got %d", len(readBars))
	}
}

// TestArenaCache_ReadNonExistentFile verifies error handling for missing files
func TestArenaCache_ReadNonExistentFile(t *testing.T) {
	cleanup := setupArenaCacheTest(t)
	defer cleanup()

	_, _, err := readArenaOHLCVFile("NONEXISTENT", "60m")
	if err == nil {
		t.Error("expected error when reading non-existent file")
	}
}

// TestArenaCache_EmptyBarsNotCached verifies that getArenaOHLCVFromMemCache
// returns false for empty bar slices
func TestArenaCache_EmptyBarsNotCached(t *testing.T) {
	cleanup := setupArenaCacheTest(t)
	defer cleanup()

	// Write empty bars to memory
	arenaOHLCVMemCacheMu.Lock()
	arenaOHLCVMemCache["EMPTY"] = map[string]*ohlcvCacheEntry{
		"60m": {Bars: []OHLCV{}, LastAccess: time.Now()},
	}
	arenaOHLCVMemCacheMu.Unlock()

	_, ok := getArenaOHLCVFromMemCache("EMPTY", "60m")
	if ok {
		t.Error("expected empty bars to return false")
	}
}
