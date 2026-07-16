//go:build windows

package fingerprint

import (
	"os/exec"
	"strings"

	"golang.org/x/sys/windows/registry"
)

func osSpecificSources() []Source {
	if value := windowsComputerUUID(); value != "" {
		return []Source{{
			Name:  "computer_uuid",
			Value: value,
		}}
	}

	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Cryptography`, registry.QUERY_VALUE)
	if err != nil {
		return nil
	}
	defer key.Close()

	value, _, err := key.GetStringValue("MachineGuid")
	if err != nil {
		return nil
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}

	return []Source{{
		Name:  "machine_guid",
		Value: value,
	}}
}

func windowsComputerUUID() string {
	commands := [][]string{
		{"wmic", "csproduct", "get", "UUID"},
		{"powershell", "-NoProfile", "-Command", "(Get-CimInstance Win32_ComputerSystemProduct).UUID"},
	}

	for _, args := range commands {
		if len(args) == 0 {
			continue
		}
		output, err := exec.Command(args[0], args[1:]...).Output()
		if err != nil {
			continue
		}
		value := normalizeWindowsUUIDOutput(string(output))
		if value != "" {
			return value
		}
	}

	return ""
}

func normalizeWindowsUUIDOutput(output string) string {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		value := strings.TrimSpace(line)
		if value == "" {
			continue
		}
		if strings.EqualFold(value, "uuid") {
			continue
		}
		if strings.EqualFold(value, "ffffffff-ffff-ffff-ffff-ffffffffffff") {
			continue
		}
		return value
	}

	return ""
}
