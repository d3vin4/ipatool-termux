//go:build darwin || linux || android

package unicorn

import "fmt"

func openArchiveEntry(path, name string, format archiveFormat) (archiveEntry, error) {
	if format != archiveZIP {
		return archiveEntry{}, fmt.Errorf("unsupported Unicorn archive format %d", format)
	}

	return openZIPEntry(path, name)
}
