package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Variable/constant/package names follow Go Naming Conventions (camelCase).
var (
	syncStrategy         string
	sourceDirectory      string
	destinationDirectory string
)

func init() {
	// 1. Input parameters --source-directory, --destination-directory
	// Use hyphens in flag names for CLI Best Practices,
	// although in Go code it is `camelCase`.
	flag.StringVar(&syncStrategy, "strategy", "mirror", "Synchronization strategy: 'mirror' (Source is mirrored to Destination, files deleted) or 'merge' (Source is copied/merged into Destination, no files deleted).")
	flag.StringVar(&sourceDirectory, "source-directory", "", "Source directory to watch and synchronize.")
	flag.StringVar(&destinationDirectory, "destination-directory", "", "Destination directory for synchronization.")

}

func validateInputs() error {
	if sourceDirectory == "" || destinationDirectory == "" {
		return fmt.Errorf("both --source-directory and --destination-directory must be specified")
	}
	// Checking the correctness of the strategy
	if syncStrategy != "mirror" && syncStrategy != "merge" {
		return fmt.Errorf("invalid strategy specified. Use 'mirror' or 'merge'")
	}
	return nil
}

func main() {
	runtime.GOMAXPROCS(1)
	flag.Parse()

	if err := validateInputs(); err != nil {
		fmt.Printf("Error: %v\n", err)
		flag.Usage()
		os.Exit(1)
	}

	// Cleaning up paths
	sourceDirectory = filepath.Clean(sourceDirectory)
	destinationDirectory = filepath.Clean(destinationDirectory)

	sourceDirectoryExists, err := directoryExists(sourceDirectory)
	if err != nil {
		log.Fatalf("Error checking directory: %v", err)
	}
	if !sourceDirectoryExists {
		log.Fatalf("FATAL: Source directory \"%s\" does not exist or is a file. The application cannot start.", sourceDirectory)
	}

	destinationDirectoryExists, err := directoryExists(destinationDirectory)
	if err != nil {
		log.Fatalf("Error checking directory: %v", err)
	}
	if !destinationDirectoryExists {
		log.Fatalf("FATAL: Destination directory \"%s\" does not exist or is a file. The application cannot start.", destinationDirectory)
	}

	log.Printf("Starting Syncer: Source=%s, Destination=%s", sourceDirectory, destinationDirectory)

	// First sync (full)
	log.Println("Performing initial full synchronization...")
	if err := syncDirectory(sourceDirectory, destinationDirectory); err != nil {
		log.Fatalf("Initial sync failed: %v", err)
	}
	log.Println("Initial synchronization complete. Starting file watcher.")

	// 2. Stay in processes constantly and monitor changes
	if err := startWatcher(sourceDirectory, destinationDirectory); err != nil {
		log.Fatalf("Watcher failed: %v", err)
	}
}

// 2. Stay in processes constantly and monitor changes
func startWatcher(source, destination string) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	done := make(chan bool)

	// Goroutine for handling events
	go func() {
		// Use debounce to aggregate fast events
		var debounceTimer *time.Timer
		const debounceDuration = 1000 * time.Millisecond // 50 ms to reduce load

		// Semaphore to ensure only one sync runs at a time
		syncSemaphore := make(chan struct{}, 1)

		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}

				// Logic for adding new directories to the trace
				if event.Op&fsnotify.Create == fsnotify.Create {
					fileInfo, err := os.Stat(event.Name)
					if err == nil && fileInfo.IsDir() {
						// Recursively add a new directory and its subdirectories
						_ = filepath.Walk(event.Name, func(path string, info os.FileInfo, err error) error {
							if info != nil && info.IsDir() {
								if err := watcher.Add(path); err != nil {
									log.Printf("Warning: Failed to add path %s to watcher: %v", path, err)
								}
							}
							return nil
						})
					}
				}

				// Start synchronization with debounce
				if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename) != 0 {
					// event.Name can be from source or destination.
					// Always run syncDirectory (Source -> Destination)

					log.Printf("Detected change in *any* monitored directory: %s %s. Scheduling sync...", event.Op.String(), event.Name)

					if debounceTimer != nil {
						debounceTimer.Stop()
					}

					// Delay synchronization for a short time
					debounceTimer = time.AfterFunc(debounceDuration, func() {
						select {
						case syncSemaphore <- struct{}{}:
							log.Println("Starting synchronization.")
							defer func() { <-syncSemaphore }() // Release semaphore
							if err := syncDirectory(source, destination); err != nil {
								// FATAL: If sync fails (e.g., disk disconnected), stop the whole application.
								log.Fatalf("Synchronization failed, stopping application: %v", err)
							} else {
								log.Println("Synchronization complete.")
							}
						default:
							log.Println("Synchronization already in progress, skipping this trigger.")
						}
					})
				}

			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Println("Watcher error:", err)
			}
		}
	}()

	// --- 3. ADDING DIRECTORIES TO TRACKING ---
	log.Println("Adding directories to watcher (Source)...")
	if err := addDirectoriesToWatcher(watcher, source); err != nil {
		return fmt.Errorf("error walking the source directory: %w", err)
	}

	if syncStrategy == "mirror" {
		log.Println("Adding directories to watcher (Destination)...")
		// AN EXTRA STEP: Recursively track Destination
		if err := addDirectoriesToWatcher(watcher, destination); err != nil {
			log.Printf("Warning: Failed to fully watch destination directory %s: %v", destination, err)
			// Not a fatal error, let's continue
		}
	} else {
		log.Println("Strategy: MERGE. Only watching Source directory.")
	}
	<-done // 2. Hang in processes constantly
	return nil
}

// New helper function for recursively adding directories
func addDirectoriesToWatcher(watcher *fsnotify.Watcher, root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Printf("Warning: access error for path %s: %v", path, err)
			// Skip this directory, but don't stop Walk
			if info != nil && info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if info.IsDir() {
			if err := watcher.Add(path); err != nil {
				log.Printf("Warning: Failed to add path %s to watcher: %v", path, err)
			}
		}
		return nil
	})
}

// 7. The speed of the script is as fast as possible
func syncDirectory(source, destination string) error {
	var wg sync.WaitGroup
	errCh := make(chan error, 100) // Buffered error channel

	// Use Walk for recursive traversal
	err := filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err // Stop if there is a fatal error
		}

		// Calculate the destination path
		relPath, err := filepath.Rel(source, path)
		if err != nil {
			return fmt.Errorf("error calculating relative path for %s: %w", path, err)
		}
		destPath := filepath.Join(destination, relPath)

		// Check if the file/directory exists in the destination
		destInfo, err := os.Stat(destPath)
		existsInDest := !os.IsNotExist(err)

		if info.IsDir() {
			// Create the directory if it does not exist
			if !existsInDest {
				if err := os.MkdirAll(destPath, info.Mode()); err != nil {
					return fmt.Errorf("failed to create directory %s: %w", destPath, err)
				}
				// Set access rights and modification time for the directory
				setDirMetadata(destPath, info)
			}
			return nil
		}

		// 6. Synchronization should be similar to cp --preserve=all
		if existsInDest && info.ModTime().Equal(destInfo.ModTime()) && info.Size() == destInfo.Size() {
			// The file is the same (simple heuristics), skip it
			return nil
		}

		// 8. Use Go multithreading (Goroutine) for copying
		//wg.Add(1)
		//go func(src, dest string, srcInfo os.FileInfo) {
		//	defer wg.Done()
		log.Printf("Copying/Updating: %s -> %s", path, destPath)
		if err := copyFile(path, destPath, info); err != nil {
			//errCh <- fmt.Errorf("failed to copy %s: %w", src, err)
		}
		//}(path, destPath, info)

		return nil
	})

	if err != nil {
		return err
	}

	// Wait for all copying Goroutines to complete
	//wg.Wait()

	// Check for errors from Goroutines
	select {
	case err := <-errCh:
		return err // Return the first error
	default:
		// If there are no errors, continue
	}

	// Additional step: Delete files/folders in destination that are not in source
	// This ensures IDENTITY (like rsync --delete or full cp/rsync)
	if syncStrategy == "mirror" { // Delete only for the mirror strategy
		log.Println("Checking destination for files to remove...")
		err = filepath.Walk(destination, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			relPath, err := filepath.Rel(destination, path)
			if err != nil {
				return err
			}
			if relPath == "." {
				return nil // Skip the root directory
			}

			sourcePath := filepath.Join(source, relPath)

			// Check for existence in source
			if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
				//wg.Add(1)
				//go func(p string) {
				//	defer wg.Done()
				log.Printf("Deleting from destination: %s", path)
				// Delete recursively if it's a directory
				if err := os.RemoveAll(path); err != nil {
					//errCh <- fmt.Errorf("failed to delete %s: %w", p, err)
				}
				//}(path)

				if info.IsDir() {
					return filepath.SkipDir // Skip traversal of the deleted directory
				}
			}
			return nil
		})

		wg.Wait()

		select {
		case err := <-errCh:
			return err
		default:
			return err // Return an error from Walk, if there was one
		}
	} else {
		log.Println("Strategy: MERGE. Skipping file deletion in Destination.")
	}

	return nil // If strategy == merge, return nil after copying
}

// 6. Function for copying a file with saving metadata
func copyFile(src, dst string, info os.FileInfo) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	// Create a temporary file in the destination directory to make the copy more atomic
	tempFile, err := os.CreateTemp(filepath.Dir(dst), "syncer-tmp-")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}

	// Copy content to temporary file
	_, err = io.Copy(tempFile, sourceFile)

	// Always close the temp file
	closeErr := tempFile.Close()

	if err != nil {
		os.Remove(tempFile.Name()) // Clean up temp file on copy error
		return fmt.Errorf("failed to copy to temp file: %w", err)
	}

	if closeErr != nil {
		os.Remove(tempFile.Name()) // Clean up temp file on close error
		return fmt.Errorf("failed to close temp file: %w", closeErr)
	}

	// If copy and close were successful, rename temp file to final destination.
	// This is an atomic operation on most local filesystems.
	if err := os.Rename(tempFile.Name(), dst); err != nil {
		os.Remove(tempFile.Name()) // Clean up temp file on rename error
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	// 6. Saving access rights (chmod) - This can fail on non-POSIX filesystems like exFAT.
	if err := os.Chmod(dst, info.Mode()); err != nil {
		log.Printf("Warning: Failed to set permissions on %s: %v", dst, err)
	}

	// 6. Saving modification time (utimes) - This can also fail on some filesystems.
	// Saving access and modification time
	if err := os.Chtimes(dst, time.Now(), info.ModTime()); err != nil {
		log.Printf("Warning: Failed to set modification time on %s: %v", dst, err)
	}

	// Attempt to save owner/group information (Chown)
	// This may cause an "operation not permitted" error without sudo
	// fileStat := info.Sys().(*syscall.Stat_t)
	// if err := os.Chown(dst, int(fileStat.Uid), int(fileStat.Gid)); err != nil {
	//     log.Printf("Warning: Failed to chown file %s: %v", dst, err)
	// }

	return nil
}

// Helper function to set metadata for directories
func setDirMetadata(path string, info os.FileInfo) {
	if err := os.Chmod(path, info.Mode()); err != nil {
		log.Printf("Warning: Failed to chmod directory %s: %v", path, err)
	}
	if err := os.Chtimes(path, time.Now(), info.ModTime()); err != nil {
		log.Printf("Warning: Failed to chtimes directory %s: %v", path, err)
	}
}

func directoryExists(path string) (bool, error) {
	info, err := os.Stat(path)

	if os.IsNotExist(err) {
		// Шлях не існує (найчистіший випадок)
		return false, nil
	}
	if err != nil {
		// Інші помилки, наприклад, помилка дозволу
		return false, err
	}

	// Перевірка, чи це каталог
	if !info.IsDir() {
		// Існує, але це файл, а не каталог
		return false, nil
	}

	// Каталог існує
	return true, nil
}
