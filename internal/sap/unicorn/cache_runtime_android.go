//go:build android

package unicorn

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// cachedRuntimePaths resolves the Unicorn runtime library on Android.
//
// The upstream artifact is a manylinux build, so the glibc linked library is
// patched for bionic once it has been downloaded and verified. The patched
// copy is cached separately and reused on later runs.
func cachedRuntimePaths(ctx context.Context) (runtimePaths, error) {
	selected, err := artifactFor("linux", runtime.GOARCH)
	if err != nil {
		return runtimePaths{}, err
	}

	cache, err := os.UserCacheDir()
	if err != nil {
		return runtimePaths{}, fmt.Errorf("locate user cache: %w", err)
	}

	root := filepath.Join(cache, "ipatool", "unicorn", unicornVersion)
	paths := runtimePaths{dependencies: make([]string, 0, len(selected.dependencies))}

	for _, dependency := range selected.dependencies {
		path, err := ensureLibrary(ctx, root, dependency, artifactHTTPClient)
		if err != nil {
			return runtimePaths{}, err
		}

		paths.dependencies = append(paths.dependencies, path)
	}

	library, err := ensureBionicLibrary(ctx, root, selected)
	if err != nil {
		return runtimePaths{}, err
	}

	paths.library, err = prepareRuntimeLibrary(library)
	if err != nil {
		return runtimePaths{}, err
	}

	return paths, nil
}
