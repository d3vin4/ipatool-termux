//go:build android

package machine

import (
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveMacAddressPersistsIdentity(t *testing.T) {
	directory := t.TempDir()
	t.Setenv("XDG_STATE_HOME", directory)

	address, err := resolveMacAddress(nil, nil)
	if err != nil {
		t.Fatalf("resolve mac address: %v", err)
	}

	if !usableMacAddress(mustParse(t, address)) {
		t.Fatalf("resolved address %q is not usable", address)
	}

	// The identity has to survive across invocations.
	again, err := resolveMacAddress(nil, nil)
	if err != nil {
		t.Fatalf("resolve mac address again: %v", err)
	}

	if again != address {
		t.Fatalf("resolved address changed from %q to %q", address, again)
	}

	stored := filepath.Join(directory, "ipatool", androidMachineIDFile)
	if _, err := os.Stat(stored); err != nil {
		t.Fatalf("identity was not persisted: %v", err)
	}
}

func TestResolveMacAddressRejectsCorruptIdentity(t *testing.T) {
	directory := t.TempDir()
	t.Setenv("XDG_STATE_HOME", directory)

	if err := os.MkdirAll(filepath.Join(directory, "ipatool"), 0o700); err != nil {
		t.Fatalf("create state directory: %v", err)
	}

	stored := filepath.Join(directory, "ipatool", androidMachineIDFile)
	if err := os.WriteFile(stored, []byte("not-a-mac-address\n"), 0o600); err != nil {
		t.Fatalf("write identity: %v", err)
	}

	address, err := resolveMacAddress(nil, nil)
	if err != nil {
		t.Fatalf("resolve mac address: %v", err)
	}

	if !usableMacAddress(mustParse(t, address)) {
		t.Fatalf("resolved address %q is not usable", address)
	}
}

func mustParse(t *testing.T, address string) net.HardwareAddr {
	t.Helper()

	parsed, err := net.ParseMAC(address)
	if err != nil {
		t.Fatalf("parse mac address %q: %v", address, err)
	}

	return parsed
}
