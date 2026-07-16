//go:build linux

package fingerprint

import (
	"os"
	"strings"
)

// osSpecificSources prefers DMI hardware identifiers and uses machine-id only
// when the kernel does not expose hardware metadata.
func osSpecificSources() []Source {
	if hardware := readLinuxSources([]struct {
		name string
		path string
	}{
		{name: "product_uuid", path: "/sys/class/dmi/id/product_uuid"},
		{name: "product_serial", path: "/sys/class/dmi/id/product_serial"},
		{name: "board_serial", path: "/sys/class/dmi/id/board_serial"},
		{name: "chassis_serial", path: "/sys/class/dmi/id/chassis_serial"},
	}); len(hardware) > 0 {
		return hardware
	}

	return readLinuxSources([]struct {
		name string
		path string
	}{
		{name: "machine_id", path: "/etc/machine-id"},
		{name: "dbus_machine_id", path: "/var/lib/dbus/machine-id"},
	})
}

func readLinuxSources(paths []struct {
	name string
	path string
}) []Source {
	items := make([]Source, 0, len(paths))
	for _, item := range paths {
		body, err := os.ReadFile(item.path)
		if err != nil {
			continue
		}
		value := strings.TrimSpace(string(body))
		if value == "" {
			continue
		}
		items = append(items, Source{Name: item.name, Value: value})
	}

	return items
}
