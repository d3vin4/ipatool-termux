//go:build !android

package fairplay

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

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
			}
		} else {
			// Link atomically fails if another writer created the destination.
			// Both names reside on the same filesystem by construction.
			err = os.Link(file.temporary, file.destination)
		}

		if err != nil {
			removeBackup(item.backup)

			return errors.Join(fmt.Errorf("publish %s: %w", file.destination, err), rollbackFiles(published))
		}

		published = append(published, item)
	}

	for _, item := range published {
		removeBackup(item.backup)
	}

	return nil
}

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
	if err := os.Link(name, backup); err != nil {
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
				err = os.Link(file.backup, file.destination)
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
