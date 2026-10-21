package fairplay

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// checkPlatform restricts the FairPlay pipeline to the platforms whose file
// semantics the private state guarantees rely on. Android is included because
// the private state lives inside the application sandbox, where the kernel
// enforces the 0600 and 0700 modes against the owning application alone.
func checkPlatform() error {
	switch runtime.GOOS {
	case "darwin", "linux", "android":
		return nil
	default:
		return fmt.Errorf("FairPlay decryption is supported only on macOS, Linux and Android; private-state permission validation is unavailable on %s", runtime.GOOS)
	}
}

// canonicalPath resolves existing ancestors even when the final path is new.
func canonicalPath(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("FairPlay file path is empty")
	}
	abs, err := filepath.Abs(name)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err == nil {
		return resolved, nil
	}
	if !os.IsNotExist(err) {
		return "", err
	}
	// A dangling symlink is not a new file and must not be treated as one.
	if info, statErr := os.Lstat(abs); statErr == nil && info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("dangling symlink: %s", abs)
	}
	parent := filepath.Dir(abs)
	if parent == abs {
		return "", err
	}
	resolved, err = canonicalPath(parent)
	if err != nil {
		return "", err
	}
	return filepath.Join(resolved, filepath.Base(abs)), nil
}

func pathsAlias(left, right string) bool {
	if left == right || (runtime.GOOS == "darwin" || runtime.GOOS == "windows") && strings.EqualFold(left, right) {
		return true
	}
	a, aErr := os.Stat(left)
	b, bErr := os.Stat(right)
	return aErr == nil && bErr == nil && os.SameFile(a, b)
}

func validatePaths(options DecryptOptions) (DecryptOptions, error) {
	inputs := []*string{&options.InputPath}
	for _, name := range inputs {
		resolved, err := canonicalPath(*name)
		if err != nil {
			return options, err
		}
		info, err := os.Stat(resolved)
		if err != nil {
			return options, err
		}
		if !info.Mode().IsRegular() {
			return options, fmt.Errorf("FairPlay input must be a regular file: %s", resolved)
		}
		*name = resolved
	}
	for _, name := range []*string{&options.OutputPath, &options.ReportPath} {
		info, err := os.Lstat(*name)
		if err != nil && !os.IsNotExist(err) {
			return options, err
		}
		if err == nil && !info.Mode().IsRegular() {
			return options, fmt.Errorf("FairPlay destination must be a regular file, not a symlink or directory: %s", *name)
		}
		resolved, err := canonicalPath(*name)
		if err != nil {
			return options, err
		}
		for _, input := range inputs {
			if pathsAlias(resolved, *input) {
				return options, fmt.Errorf("refusing to overwrite FairPlay input or private state: %s", resolved)
			}
		}
		if info != nil && !options.Force {
			return options, fmt.Errorf("refusing to replace %s", resolved)
		}
		*name = resolved
	}
	if pathsAlias(options.OutputPath, options.ReportPath) {
		return options, fmt.Errorf("FairPlay output and report paths must be distinct")
	}
	return options, nil
}

type stagedFile struct {
	temporary, destination string
}

type publishedFile struct {
	stagedFile
	info   os.FileInfo
	backup string
}

// publishFiles provides no-replace publication and rolls back ordinary I/O
// failures. Two separate directory entries cannot be committed crash-atomically.
// Destinations must reside in directories controlled by the caller.
func removeBackup(backup string) {
	if backup != "" {
		os.Remove(backup)
		os.Remove(filepath.Dir(backup))
	}
}
