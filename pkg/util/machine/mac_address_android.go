//go:build android

package machine

import (
	"crypto/rand"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
)

// androidMachineIDFile stores the locally administered hardware address that
// stands in for the real one.
const androidMachineIDFile = "machine-id"

// resolveMacAddress returns the hardware address that identifies the device.
//
// Android does not expose the hardware address of the network interfaces to
// applications: the kernel reports a dummy address for every interface and
// blocks the sysfs entries that hold the real one. When no usable address is
// available, a locally administered address is generated once and persisted
// next to the session state, so the device identity stays stable across runs
// instead of changing with every invocation.
func resolveMacAddress(interfaces []net.Interface, lookup func(net.Interface) (macAddressCandidate, error)) (string, error) {
	address, err := selectMacAddress(interfaces, lookup)
	if err == nil {
		return address, nil
	}

	return persistedMacAddress()
}

// persistedMacAddress returns the stored locally administered address, creating
// it when it does not exist yet.
func persistedMacAddress() (string, error) {
	directory := androidStateDirectory()
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return "", fmt.Errorf("create machine identity directory: %w", err)
	}

	path := filepath.Join(directory, androidMachineIDFile)

	address, err := readMachineID(path)
	if err == nil {
		return address, nil
	}

	// A stored identity that cannot be used is replaced, otherwise a
	// corrupted file would keep the tool from resolving an address.
	if _, statErr := os.Stat(path); statErr == nil {
		if removeErr := os.Remove(path); removeErr != nil {
			return "", fmt.Errorf("remove invalid machine identity: %w", removeErr)
		}
	}

	address, err = generateMachineID()
	if err != nil {
		return "", err
	}

	if err := writeMachineID(path, address); err != nil {
		// Another process may have stored the identity first.
		if existing, readErr := readMachineID(path); readErr == nil {
			return existing, nil
		}

		return "", err
	}

	return address, nil
}

// androidStateDirectory mirrors the session state layout used by the command
// layer so the identity is stored next to the other persistent state.
func androidStateDirectory() string {
	for _, variable := range []string{"XDG_STATE_HOME", "XDG_DATA_HOME"} {
		if base := os.Getenv(variable); filepath.IsAbs(base) {
			return filepath.Join(base, "ipatool")
		}
	}

	return filepath.Join(os.Getenv("HOME"), ".ipatool")
}

// readMachineID reads the persisted hardware address.
func readMachineID(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read machine identity: %w", err)
	}

	address := strings.TrimSpace(string(data))

	parsed, err := net.ParseMAC(address)
	if err != nil {
		return "", fmt.Errorf("parse machine identity: %w", err)
	}

	if !usableMacAddress(parsed) {
		return "", fmt.Errorf("machine identity %q is not a usable hardware address", address)
	}

	return parsed.String(), nil
}

// generateMachineID produces a locally administered unicast address.
func generateMachineID() (string, error) {
	for range 8 {
		buffer := make([]byte, 6)
		if _, err := rand.Read(buffer); err != nil {
			return "", fmt.Errorf("generate machine identity: %w", err)
		}

		buffer[0] |= 0x02
		buffer[0] &= 0xfe

		address := net.HardwareAddr(buffer)
		if !usableMacAddress(address) {
			continue
		}

		return address.String(), nil
	}

	return "", fmt.Errorf("generate machine identity: no usable address produced")
}

// writeMachineID stores the hardware address for later runs.
func writeMachineID(path, address string) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create machine identity: %w", err)
	}

	defer file.Close()

	if _, err := file.WriteString(address + "\n"); err != nil {
		return fmt.Errorf("write machine identity: %w", err)
	}

	return nil
}
