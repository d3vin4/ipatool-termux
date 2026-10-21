//go:build !android

package machine

import "net"

// resolveMacAddress picks the hardware address that identifies the device.
func resolveMacAddress(interfaces []net.Interface, lookup func(net.Interface) (macAddressCandidate, error)) (string, error) {
	return selectMacAddress(interfaces, lookup)
}
