//go:build android

package unicorn

import (
	"bytes"
	"context"
	"crypto/sha256"
	"debug/elf"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
)

const (
	dtNeeded     = 1
	dtVerneed    = 0x6FFFFFFE
	dtVerneedNum = 0x6FFFFFFF

	// verNdxGlobal is the symbol version index that carries no version
	// requirement.
	verNdxGlobal = 1
)

// bionicSonames maps the glibc sonames the upstream Unicorn build imports to
// the equivalents provided by Android's bionic libc. Bionic keeps the pthread
// entry points in libc, so libpthread maps to libc as well.
var bionicSonames = map[string]string{
	"libc.so.6":       "libc.so",
	"libm.so.6":       "libm.so",
	"libpthread.so.0": "libc.so",
	"libdl.so.2":      "libc.so",
	"librt.so.1":      "libc.so",
}

// bionicSymbols maps the glibc internal aliases the upstream Unicorn build
// imports to the bionic symbols with identical semantics. Bionic exposes errno
// as a function, which matches the behaviour of glibc's __errno_location.
var bionicSymbols = map[string]string{
	"__strdup":         "strdup",
	"__getauxval":      "getauxval",
	"__sigsetjmp":      "sigsetjmp",
	"__errno_location": "__errno",
}

// patchedLibrarySHA256 records the digest of the bionic compatible Unicorn
// library. The upstream artifact is pinned by checksum, so the patched output
// is deterministic and can be verified against a fixed digest.
var patchedLibrarySHA256 = map[string]string{
	"arm64": "e4ac828fecd8452671761483bfcad18a604f926d9abb9af5f98850944a98d777",
}

// patchBionic rewrites the glibc linked Unicorn library at source into a
// bionic compatible copy at destination.
func patchBionic(source, destination string) error {
	data, err := os.ReadFile(source)
	if err != nil {
		return fmt.Errorf("read Unicorn library: %w", err)
	}

	patched, err := patchBionicLibrary(data)
	if err != nil {
		return err
	}

	temporary, err := os.CreateTemp(filepath.Dir(destination), ".libunicorn-bionic-*")
	if err != nil {
		return fmt.Errorf("create patched Unicorn library: %w", err)
	}

	path := temporary.Name()

	if _, err := temporary.Write(patched); err != nil {
		_ = temporary.Close()
		_ = os.Remove(path)

		return fmt.Errorf("write patched Unicorn library: %w", err)
	}

	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		_ = os.Remove(path)

		return fmt.Errorf("sync patched Unicorn library: %w", err)
	}

	if err := temporary.Close(); err != nil {
		_ = os.Remove(path)

		return fmt.Errorf("close patched Unicorn library: %w", err)
	}

	if err := os.Chmod(path, 0o600); err != nil {
		_ = os.Remove(path)

		return fmt.Errorf("set patched Unicorn library permissions: %w", err)
	}

	if err := os.Rename(path, destination); err != nil {
		_ = os.Remove(path)

		return fmt.Errorf("install patched Unicorn library: %w", err)
	}

	return nil
}

// patchBionicLibrary rewrites a glibc linked shared object so that the bionic
// loader accepts it. Every edit happens at a byte offset that is already part
// of the file, so the ELF layout stays valid. Applying the patch twice is a
// no-op, which keeps a cached library safe to re-patch.
func patchBionicLibrary(data []byte) ([]byte, error) {
	file, err := elf.NewFile(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("parse Unicorn library: %w", err)
	}

	defer file.Close()

	dynstr := file.Section(".dynstr")
	dynamic := file.Section(".dynamic")
	versym := file.Section(".gnu.version")

	if dynstr == nil || dynamic == nil || versym == nil {
		return nil, fmt.Errorf("Unicorn library is missing the ELF sections required for the bionic patch")
	}

	blob := make([]byte, len(data))
	copy(blob, data)

	table := blob[dynstr.Offset : dynstr.Offset+dynstr.Size]

	entries, err := parseDynamicEntries(blob, dynamic)
	if err != nil {
		return nil, err
	}

	hasVerneed := false

	for _, entry := range entries {
		switch entry.tag {
		case dtNeeded:
			current, err := readCString(table, int(entry.value))
			if err != nil {
				return nil, err
			}

			replacement, ok := bionicSonames[current]
			if !ok {
				continue
			}

			if err := writeCString(blob, int(dynstr.Offset), int(entry.value), replacement); err != nil {
				return nil, err
			}
		case dtVerneed, dtVerneedNum:
			hasVerneed = true

			binary.LittleEndian.PutUint64(blob[entry.offset+8:], 0)
		}
	}

	if !hasVerneed {
		return nil, fmt.Errorf("Unicorn library does not declare a symbol version requirement")
	}

	for name, replacement := range bionicSymbols {
		if err := renameDynString(blob, int(dynstr.Offset), table, name, replacement); err != nil {
			return nil, err
		}
	}

	versions := blob[versym.Offset : versym.Offset+versym.Size]
	for index := 0; index+1 < len(versions); index += 2 {
		if binary.LittleEndian.Uint16(versions[index:]) < verNdxGlobal {
			continue
		}

		binary.LittleEndian.PutUint16(versions[index:], verNdxGlobal)
	}

	return blob, nil
}

// dynamicEntry describes a single Elf64_Dyn entry and the offset of its
// value inside the file.
type dynamicEntry struct {
	tag    uint64
	value  uint64
	offset int
}

// parseDynamicEntries reads the dynamic section of the ELF file.
func parseDynamicEntries(blob []byte, section *elf.Section) ([]dynamicEntry, error) {
	contents := blob[section.Offset : section.Offset+section.Size]

	entries := make([]dynamicEntry, 0, len(contents)/16)

	for offset := 0; offset+16 <= len(contents); offset += 16 {
		tag := binary.LittleEndian.Uint64(contents[offset:])
		value := binary.LittleEndian.Uint64(contents[offset+8:])

		entries = append(entries, dynamicEntry{
			tag:    tag,
			value:  value,
			offset: int(section.Offset) + offset,
		})

		if tag == 0 {
			break
		}
	}

	return entries, nil
}

// readCString reads the NUL terminated string stored at the given offset of
// the dynamic string table.
func readCString(table []byte, offset int) (string, error) {
	if offset < 0 || offset >= len(table) {
		return "", fmt.Errorf("dynamic string offset %d is out of range", offset)
	}

	end := bytes.IndexByte(table[offset:], 0)
	if end < 0 {
		return "", fmt.Errorf("dynamic string at offset %d is not terminated", offset)
	}

	return string(table[offset : offset+end]), nil
}

// writeCString replaces the string at the given offset of the dynamic string
// table. The replacement is padded with NUL bytes so the table keeps its
// original size and every other offset stays valid.
func writeCString(blob []byte, tableOffset int, offset int, value string) error {
	current, err := readCString(blob[tableOffset:], offset)
	if err != nil {
		return err
	}

	if len(value) > len(current) {
		return fmt.Errorf("replacement %q does not fit in %q", value, current)
	}

	start := tableOffset + offset
	copy(blob[start:start+len(value)], value)

	for index := start + len(value); index < start+len(current); index++ {
		blob[index] = 0
	}

	return nil
}

// renameDynString rewrites a symbol name stored in the dynamic string table.
// The name is located as a standalone string, so a suffix of a longer symbol
// name is never matched. When the old name is absent but the new name is
// present the library was already patched and nothing is done.
func renameDynString(blob []byte, tableOffset int, table []byte, name, replacement string) error {
	offset, found := findDynString(table, name)
	if !found {
		if _, ok := findDynString(table, replacement); ok {
			return nil
		}

		return fmt.Errorf("Unicorn library does not import %q", name)
	}

	if len(replacement) > len(name) {
		return fmt.Errorf("replacement %q does not fit in %q", replacement, name)
	}

	start := tableOffset + offset
	copy(blob[start:start+len(replacement)], replacement)

	for index := start + len(replacement); index < start+len(name); index++ {
		blob[index] = 0
	}

	return nil
}

// findDynString locates a standalone string in the dynamic string table.
func findDynString(table []byte, name string) (int, bool) {
	if len(table) > len(name) && bytes.HasPrefix(table, []byte(name)) && table[len(name)] == 0 {
		return 0, true
	}

	pattern := make([]byte, 0, len(name)+2)
	pattern = append(pattern, 0)
	pattern = append(pattern, name...)
	pattern = append(pattern, 0)

	index := bytes.Index(table, pattern)
	if index < 0 {
		return 0, false
	}

	return index + 1, true
}

// fileSHA256 returns the hex encoded SHA-256 digest of a file.
func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open checksum target: %w", err)
	}

	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("checksum target: %w", err)
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// ensureBionicLibrary returns the path of a bionic compatible copy of the
// upstream Unicorn library. The library is downloaded, verified and patched on
// first use, and the patched copy is reused on later runs.
func ensureBionicLibrary(ctx context.Context, root string, selected artifact) (string, error) {
	directory := filepath.Join(root, selected.librarySHA256, "bionic")
	destination := filepath.Join(directory, selected.filename)

	expected, known := patchedLibrarySHA256[runtime.GOARCH]
	if known {
		valid, err := fileMatchesSHA256(destination, expected)
		if err == nil && valid {
			return destination, nil
		}
	} else if _, err := os.Stat(destination); err == nil {
		return destination, nil
	}

	if err := os.MkdirAll(directory, 0o700); err != nil {
		return "", fmt.Errorf("create bionic Unicorn cache: %w", err)
	}

	archive, err := downloadArtifact(ctx, directory, selected, artifactHTTPClient)
	if err != nil {
		return "", err
	}

	defer os.Remove(archive)

	extracted, err := extractLibrary(archive, directory, selected)
	if err != nil {
		return "", err
	}

	defer os.Remove(extracted)

	if err := patchBionic(extracted, destination); err != nil {
		return "", err
	}

	if !known {
		return destination, nil
	}

	digest, err := fileSHA256(destination)
	if err != nil {
		return "", fmt.Errorf("checksum patched Unicorn library: %w", err)
	}

	if digest != expected {
		return "", fmt.Errorf("patched Unicorn library SHA-256 is %s, want %s", digest, expected)
	}

	return destination, nil
}
