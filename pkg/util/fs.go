// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package util

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// CopyDir recursively copies a directory tree, attempting to preserve permissions.
// Source directory must exist.
func CopyDir(src string, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		target := filepath.Join(dst, rel)

		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		return os.WriteFile(target, data, info.Mode())
	})
}

// CopyFile copies a single file from src to dst.
func CopyFile(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	return os.WriteFile(dst, data, info.Mode())
}

// MakeWritableRecursive recursively makes all files and directories in the path writable by the user.
func MakeWritableRecursive(path string) error {
	var totalFiles, chmodCount int
	err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		totalFiles++
		if info.Mode().Perm()&0200 == 0 {
			chmodCount++
			return os.Chmod(path, info.Mode().Perm()|0200)
		}
		return nil
	})
	Debugf("MakeWritableRecursive: walked %d files, chmod'd %d", totalFiles, chmodCount)
	return err
}

// RemoveAllAsync removes a directory tree without blocking the caller.
// It renames the directory to a unique tombstone name (an instant metadata
// operation on the same filesystem), then runs the actual deletion in a
// background goroutine using removeAllSafe. This avoids blocking on
// slow-to-delete files such as symlinks pointing to container-internal
// paths (e.g. /home/scion/...) which can trigger macOS autofs timeouts.
func RemoveAllAsync(path string) error {
	tombstone := fmt.Sprintf("%s.deleting-%d", path, time.Now().UnixNano())
	if err := os.Rename(path, tombstone); err != nil {
		Debugf("RemoveAllAsync: rename failed, falling back to sync removal: %v", err)
		return removeAllSafe(path)
	}

	Debugf("RemoveAllAsync: renamed %s -> %s", filepath.Base(path), filepath.Base(tombstone))
	go func() {
		if err := removeAllSafe(tombstone); err != nil {
			Debugf("RemoveAllAsync: background removal failed: %v", err)
		}
	}()
	return nil
}

// removeAllSafe removes a directory tree in a single pass, handling
// symlinks and permissions inline to avoid the overhead of separate walks.
//
// It works in three phases:
//  1. Walk the tree with filepath.WalkDir (uses Lstat, never follows
//     symlinks). During the walk, symlinks are removed immediately and
//     read-only directories are made writable so their contents can be
//     listed and deleted.
//  2. Regular files collected during the walk are deleted.
//  3. Directories are removed bottom-up (deepest first).
//
// Stripping symlinks during phase 1 prevents macOS autofs from attempting
// to resolve dangling container-internal symlink targets (e.g.
// /home/scion/...) during the subsequent unlink calls.
func removeAllSafe(root string) error {
	var files []string
	var dirs []string
	var firstErr error

	walkErr := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			if os.IsPermission(err) {
				// Make the parent directory writable and retry.
				parent := filepath.Dir(path)
				if chErr := os.Chmod(parent, 0700); chErr == nil {
					// Re-stat to determine the entry type.
					info, stErr := os.Lstat(path)
					if stErr != nil {
						return nil // skip this entry
					}
					if info.Mode()&os.ModeSymlink != 0 {
						os.Remove(path)
						return nil
					}
					if info.IsDir() {
						os.Chmod(path, 0700)
						dirs = append(dirs, path)
						return nil
					}
					files = append(files, path)
					return nil
				}
			}
			return nil // skip entries we cannot access
		}

		if d.Type()&os.ModeSymlink != 0 {
			// Remove symlinks immediately during the walk to prevent
			// any later operation from touching their targets.
			if rmErr := os.Remove(path); rmErr != nil && os.IsPermission(rmErr) {
				os.Chmod(filepath.Dir(path), 0700)
				os.Remove(path)
			}
			return nil
		}

		if d.IsDir() {
			// Ensure the directory is writable so we can list/delete contents.
			info, infoErr := d.Info()
			if infoErr == nil && info.Mode().Perm()&0700 != 0700 {
				os.Chmod(path, 0700)
			}
			dirs = append(dirs, path)
			return nil
		}

		files = append(files, path)
		return nil
	})
	if walkErr != nil && firstErr == nil {
		firstErr = walkErr
	}

	// Phase 2: remove regular files.
	for _, f := range files {
		if err := os.Remove(f); err != nil && !os.IsNotExist(err) {
			if os.IsPermission(err) {
				os.Chmod(filepath.Dir(f), 0700)
				err = os.Remove(f)
			}
			if err != nil && !os.IsNotExist(err) && firstErr == nil {
				firstErr = err
			}
		}
	}

	// Phase 3: remove directories bottom-up (deepest paths first).
	// Since WalkDir visits in lexical order (parent before children),
	// reversing gives us children before parents.
	for i := len(dirs) - 1; i >= 0; i-- {
		if err := os.Remove(dirs[i]); err != nil && !os.IsNotExist(err) {
			if os.IsPermission(err) {
				os.Chmod(dirs[i], 0700)
				err = os.Remove(dirs[i])
			}
			if err != nil && !os.IsNotExist(err) && firstErr == nil {
				firstErr = err
			}
		}
	}

	return firstErr
}

// CleanupPendingDeletions removes leftover tombstone directories in dir
// from previous async deletions that may not have completed.
func CleanupPendingDeletions(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !strings.Contains(e.Name(), ".deleting-") {
			continue
		}
		tombstone := filepath.Join(dir, e.Name())
		Debugf("CleanupPendingDeletions: removing leftover %s", e.Name())
		go removeAllSafe(tombstone)
	}
}
