package main

import (
    "os"
    "path/filepath"
    "testing"
    "time"
)

// Helper struct for managing test directories
type TestDirs struct {
    Source      string
    Destination string
    Cleanup     func()
}

// Helper to create temporary source and destination directories for testing
func setupTestDirs(t *testing.T) TestDirs {
    // 1. Create a base temporary directory
    baseDir, err := os.MkdirTemp("", "syncer_test_base")
    if err != nil {
        t.Fatalf("Failed to create base temp dir: %v", err)
    }

    // 2. Define Source and Destination paths
    source := filepath.Join(baseDir, "source")
    destination := filepath.Join(baseDir, "destination")

    // 3. Create Source and Destination directories
    if err := os.MkdirAll(source, 0755); err != nil {
        os.RemoveAll(baseDir)
        t.Fatalf("Failed to create source dir: %v", err)
    }
    if err := os.MkdirAll(destination, 0755); err != nil {
        os.RemoveAll(baseDir)
        t.Fatalf("Failed to create destination dir: %v", err)
    }

    // 4. Cleanup function
    cleanup := func() {
        if err := os.RemoveAll(baseDir); err != nil {
            t.Logf("Warning: Failed to clean up temp dir %s: %v", baseDir, err)
        }
    }

    return TestDirs{Source: source, Destination: destination, Cleanup: cleanup}
}

// Helper to create a file with specific content
func createFile(t *testing.T, path string, content string) {
    if err := os.WriteFile(path, []byte(content), 0644); err != nil {
        t.Fatalf("Failed to create file %s: %v", path, err)
    }
}

// Helper to check if a file exists
func assertFileExists(t *testing.T, path string, expected bool) {
    _, err := os.Stat(path)
    exists := !os.IsNotExist(err)
    if exists != expected {
        if expected {
            t.Errorf("FAIL: Expected file to exist at %s, but it does not.", path)
        } else {
            t.Errorf("FAIL: Expected file NOT to exist at %s, but it does.", path)
        }
    }
}

// --- TEST 1: MIRROR STRATEGY ---
func TestSyncStrategyMirror(t *testing.T) {
    // Set strategy globally for the test environment
    syncStrategy = "mirror"

    td := setupTestDirs(t)
    defer td.Cleanup()

    // 1. Initial setup:
    // Source: A.txt, B.txt
    // Dest:   A.txt (OLD), C.txt (EXTRA)
    createFile(t, filepath.Join(td.Source, "A.txt"), "Content A V1")
    createFile(t, filepath.Join(td.Source, "B.txt"), "Content B")

    // Create C.txt in Destination only (should be deleted)
    createFile(t, filepath.Join(td.Destination, "C.txt"), "Content C (Extra)")
    // Create A.txt in Destination (should be overwritten)
    createFile(t, filepath.Join(td.Destination, "A.txt"), "Content A Old")

    // Ensure B.txt is created in a subdirectory in Source
    subDirSource := filepath.Join(td.Source, "sub")
    subDirDest := filepath.Join(td.Destination, "sub")
    if err := os.Mkdir(subDirSource, 0755); err != nil {
        t.Fatalf("mkdir failed: %v", err)
    }
    createFile(t, filepath.Join(subDirSource, "D.txt"), "Content D")

    // Create Z.txt in a subdirectory in Destination only (should be deleted)
    if err := os.Mkdir(subDirDest, 0755); err != nil {
        t.Fatalf("mkdir failed: %v", err)
    }
    createFile(t, filepath.Join(subDirDest, "Z.txt"), "Content Z (Extra)")

    // Introduce a delay to ensure modification times are different
    time.Sleep(1 * time.Millisecond)

    // 2. Perform synchronization
    t.Logf("Running syncDirectory with strategy: %s", syncStrategy)
    if err := syncDirectory(td.Source, td.Destination); err != nil {
        t.Fatalf("syncDirectory failed: %v", err)
    }

    // 3. Verification (MIRROR)
    t.Run("A. Source files copied/overwritten", func(t *testing.T) {
        assertFileExists(t, filepath.Join(td.Destination, "A.txt"), true)
        assertFileExists(t, filepath.Join(td.Destination, "B.txt"), true)
        assertFileExists(t, filepath.Join(subDirDest, "D.txt"), true)

        // Check content of A.txt (should be V1, proving overwrite)
        content, _ := os.ReadFile(filepath.Join(td.Destination, "A.txt"))
        if string(content) != "Content A V1" {
            t.Errorf("A.txt content was not updated/overwritten.")
        }
    })

    t.Run("B. Extra Destination files deleted", func(t *testing.T) {
        // C.txt should be deleted
        assertFileExists(t, filepath.Join(td.Destination, "C.txt"), false)
        // Z.txt (in subdirectory) should be deleted
        assertFileExists(t, filepath.Join(subDirDest, "Z.txt"), false)
    })
}

// --- TEST 2: MERGE STRATEGY ---
func TestSyncStrategyMerge(t *testing.T) {
    // Set strategy globally for the test environment
    syncStrategy = "merge"

    td := setupTestDirs(t)
    defer td.Cleanup()

    // 1. Initial setup:
    // Source: F.txt, G.txt
    // Dest:   F.txt (OLD), H.txt (EXTRA)
    createFile(t, filepath.Join(td.Source, "F.txt"), "Content F V1")
    createFile(t, filepath.Join(td.Source, "G.txt"), "Content G")

    // Create H.txt in Destination only (SHOULD *NOT* BE DELETED)
    createFile(t, filepath.Join(td.Destination, "H.txt"), "Content H (Extra)")
    // Create F.txt in Destination (should be overwritten)
    createFile(t, filepath.Join(td.Destination, "F.txt"), "Content F Old")

    // Introduce a delay
    time.Sleep(1 * time.Millisecond)

    // 2. Perform synchronization
    t.Logf("Running syncDirectory with strategy: %s", syncStrategy)
    if err := syncDirectory(td.Source, td.Destination); err != nil {
        t.Fatalf("syncDirectory failed: %v", err)
    }

    // 3. Verification (MERGE)
    t.Run("A. Source files copied/overwritten", func(t *testing.T) {
        assertFileExists(t, filepath.Join(td.Destination, "F.txt"), true)
        assertFileExists(t, filepath.Join(td.Destination, "G.txt"), true)

        // Check content of F.txt (should be V1, proving update)
        content, _ := os.ReadFile(filepath.Join(td.Destination, "F.txt"))
        if string(content) != "Content F V1" {
            t.Errorf("F.txt content was not updated/overwritten.")
        }
    })

    t.Run("B. Extra Destination files NOT deleted", func(t *testing.T) {
        // H.txt should still exist
        assertFileExists(t, filepath.Join(td.Destination, "H.txt"), true)
        // Check content of H.txt (should be the original extra content)
        content, _ := os.ReadFile(filepath.Join(td.Destination, "H.txt"))
        if string(content) != "Content H (Extra)" {
            t.Errorf("H.txt content was unexpectedly modified or corrupted.")
        }
    })
}
