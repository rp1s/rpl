//go:build darwin

package fingerprint

import (
	"bytes"
	"os/exec"
	"regexp"
	"strings"
)

var ioPlatformUUIDPattern = regexp.MustCompile(`"IOPlatformUUID"\s*=\s*"([^"]+)"`)

// osSpecificSources prefers IOPlatformUUID because it is the most stable
// machine-level identifier commonly exposed on macOS.
func osSpecificSources() []Source {
	items := make([]Source, 0, 1)

	cmd := exec.Command("ioreg", "-rd1", "-c", "IOPlatformExpertDevice")
	output, err := cmd.Output()
	if err == nil {
		if matches := ioPlatformUUIDPattern.FindSubmatch(bytes.TrimSpace(output)); len(matches) == 2 {
			value := strings.TrimSpace(string(matches[1]))
			if value != "" {
				items = append(items, Source{Name: "platform_uuid", Value: value})
			}
		}
	}

	return items
}
