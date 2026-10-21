//go:build android

package unicorn

import (
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestPatchBionicFixture patches a glibc linked Unicorn library and compares
// the result with the known digest. The fixture is not part of the repository,
// so the test is skipped unless its location is provided.
func TestPatchBionicFixture(t *testing.T) {
	fixture := os.Getenv("IPATOOL_UNICORN_FIXTURE")
	if fixture == "" {
		t.Skip("IPATOOL_UNICORN_FIXTURE is not set")
	}

	source, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	patched, err := patchBionicLibrary(source)
	if err != nil {
		t.Fatalf("patch library: %v", err)
	}

	destination := filepath.Join(t.TempDir(), "libunicorn.so.2")
	if err := os.WriteFile(destination, patched, 0o600); err != nil {
		t.Fatalf("write patched library: %v", err)
	}

	digest, err := fileSHA256(destination)
	if err != nil {
		t.Fatalf("checksum patched library: %v", err)
	}

	expected, known := patchedLibrarySHA256["arm64"]
	if !known {
		t.Fatal("no expected digest for arm64")
	}

	if digest != expected {
		t.Fatalf("patched digest is %s, want %s", digest, expected)
	}

	// Patching an already patched library must not change it.
	again, err := patchBionicLibrary(patched)
	if err != nil {
		t.Fatalf("re-patch library: %v", err)
	}

	if string(again) != string(patched) {
		t.Fatal("patching twice changed the library")
	}
}

// TestEngineEmulation loads the runtime library through the regular code path
// and executes a small ARM64 program to prove the engine works.
func TestEngineEmulation(t *testing.T) {
	if os.Getenv("IPATOOL_UNICORN_EMULATION") == "" {
		t.Skip("IPATOOL_UNICORN_EMULATION is not set")
	}

	engine, err := New(context.Background())
	if err != nil {
		t.Fatalf("create engine: %v", err)
	}

	defer engine.Close()

	const base = 0x1000

	if err := engine.MemMap(base, 0x1000); err != nil {
		t.Fatalf("map memory: %v", err)
	}

	// mov x0, #42  (the engine emulates arm64)
	code := make([]byte, 4)
	binary.LittleEndian.PutUint32(code, 0xd2800A80)

	if err := engine.MemWrite(base, code); err != nil {
		t.Fatalf("write code: %v", err)
	}

	if err := engine.StartBounded(base, base+uint64(len(code)), 5*time.Second, 1); err != nil {
		t.Fatalf("emulate: %v", err)
	}

	value, err := engine.RegRead(RegX0)
	if err != nil {
		t.Fatalf("read register: %v", err)
	}

	if value != 42 {
		t.Fatalf("register holds %d, want 42", value)
	}
}
