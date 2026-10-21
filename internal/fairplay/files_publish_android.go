//go:build android

package fairplay

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// Android denies hard links to unprivileged applications, so the no-replace
// publication is built on renameat2(RENAME_NOREPLACE) instead. The rename
// moves the staged file onto the destination, which is equivalent to the link
// based publication for every observable purpose: the destination receives the
// content atomically and the operation fails when the destination already
// exists.
func renameNoReplace(source, destination string) error {
	return unix.Renameat2(unix.AT_FDCWD, source, unix.AT_FDCWD, destination, unix.RENAME_NOREPLACE)
}

// publishFiles provides no-replace publication and rolls back ordinary I/O
// failures. Two separate directory entries cannot be committed crash-atomically.
// Destinations must reside in directories controlled by the caller.
func publishFiles(files []stagedFile, force bool) error {
	var published []publishedFile

	for _, file := range files {
		item := publishedFile{stagedFile: file}
		info, err := os.Stat(file.temporary)
		if err != nil {
			return errors.Join(err, rollbackFiles(published))
		}

		item.info = info

		if force {
			item.backup, err = backupDestination(file.destination)
			if err == nil {
				err = os.Rename(file.temporary, file.destination)
				if err != nil {
					// The previous file was moved aside, so it has to be put
					// back before the failure is reported.
					if restoreErr := os.Rename(item.backup, file.destination); restoreErr == nil {
						removeBackup(item.backup)

						item.backup = ""
					}
				}
			}
		} else {
			// The rename fails if another writer created the destination.
			// Both names reside on the same filesystem by construction.
			err = renameNoReplace(file.temporary, file.destination)
		}

		if err != nil {
			if item.backup != "" {
				// The backup holds the only remaining copy of the previous
				// file, so it is kept instead of being discarded.
				return errors.Join(
					fmt.Errorf("publish %s: %w (previous file retained at %q)", file.destination, err, item.backup),
					rollbackFiles(published),
				)
			}

			return errors.Join(fmt.Errorf("publish %s: %w", file.destination, err), rollbackFiles(published))
		}

		published = append(published, item)
	}

	for _, item := range published {
		removeBackup(item.backup)
	}

	return nil
}

// backupDestination moves the previous file aside so that it can be restored
// when the replacement fails.
func backupDestination(name string) (string, error) {
	info, err := os.Lstat(name)
	if os.IsNotExist(err) {
		return "", nil
	}

	if err != nil {
		return "", err
	}

	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("refusing to replace non-regular destination: %s", name)
	}

	directory, err := os.MkdirTemp(filepath.Dir(name), ".fairplay-backup-*")
	if err != nil {
		return "", err
	}

	backup := filepath.Join(directory, "previous")
	if err := os.Rename(name, backup); err != nil {
		os.Remove(directory)

		return "", err
	}

	return backup, nil
}

func rollbackFiles(files []publishedFile) error {
	var failures []error

	for index := len(files) - 1; index >= 0; index-- {
		file := files[index]

		info, err := os.Lstat(file.destination)
		if err == nil && !os.SameFile(info, file.info) {
			err = errors.New("destination changed during publication")
		} else if err == nil {
			if file.backup != "" {
				err = os.Rename(file.backup, file.destination)
			} else {
				err = os.Remove(file.destination)
			}
		} else if os.IsNotExist(err) {
			err = nil

			if file.backup != "" {
				err = os.Rename(file.backup, file.destination)
			}
		}

		if err != nil {
			failures = append(failures, fmt.Errorf("rollback %s (previous version retained at %q): %w", file.destination, file.backup, err))
		} else {
			removeBackup(file.backup)
		}
	}

	return errors.Join(failures...)
}
