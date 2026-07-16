package fingerprint

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net"
	"sort"
	"strings"
)

// Source is a raw fingerprint input collected from the current device.
type Source struct {
	Name  string
	Value string
}

// Fingerprint hashes the best available machine identifiers on every call.
// It does not persist anything to disk, so the same hardware should produce
// the same value as long as the underlying system identifiers stay unchanged.
func Fingerprint() (string, error) {
	sources, err := Sources()
	if err != nil {
		return "", err
	}

	return fingerprintFromSources(sources)
}

// Sources returns the normalized raw fingerprint sources used for hashing.
func Sources() ([]Source, error) {
	sources := selectFingerprintSources(
		normalizeSources(osSpecificSources()),
		normalizeSources(commonSources()),
	)
	if len(sources) == 0 {
		return nil, errors.New("device fingerprint sources are unavailable")
	}

	return sources, nil
}

func selectFingerprintSources(primary []Source, fallback []Source) []Source {
	if len(primary) > 0 {
		return primary
	}
	return fallback
}

// fingerprintFromSources hashes a sorted "name + value" stream so the same
// device data always yields the same fingerprint regardless of source order.
func fingerprintFromSources(sources []Source) (string, error) {
	sources = normalizeSources(sources)
	if len(sources) == 0 {
		return "", errors.New("device fingerprint sources are unavailable")
	}

	hasher := sha256.New()
	for _, source := range sources {
		_, _ = hasher.Write([]byte(source.Name))
		_, _ = hasher.Write([]byte{'\n'})
		_, _ = hasher.Write([]byte(source.Value))
		_, _ = hasher.Write([]byte{'\n'})
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// normalizeSources removes empty entries, deduplicates repeated signals, and
// sorts the final set to keep hashing deterministic across runs.
func normalizeSources(sources []Source) []Source {
	items := make([]Source, 0, len(sources))
	seen := make(map[string]struct{})

	for _, source := range sources {
		name := strings.TrimSpace(source.Name)
		value := strings.TrimSpace(source.Value)
		if name == "" || value == "" {
			continue
		}

		key := name + "\x00" + value
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		items = append(items, Source{
			Name:  name,
			Value: value,
		})
	}

	sort.Slice(items, func(i int, j int) bool {
		if items[i].Name == items[j].Name {
			return items[i].Value < items[j].Value
		}
		return items[i].Name < items[j].Name
	})

	return items
}

// commonSources is a hardware-oriented fallback for systems where a dedicated
// machine identifier is not available.
func commonSources() []Source {
	sources := make([]Source, 0, 1)
	if macs := macAddresses(); len(macs) > 0 {
		for _, mac := range macs {
			sources = append(sources, Source{Name: "mac", Value: mac})
		}
	}

	return sources
}

// macAddresses filters out loopback interfaces and returns a normalized,
// deduplicated MAC list so network ordering does not affect the fingerprint.
func macAddresses() []string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil
	}

	items := make([]string, 0, len(interfaces))
	seen := make(map[string]struct{})
	for _, iface := range interfaces {
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		if len(iface.HardwareAddr) == 0 {
			continue
		}

		value := strings.ToLower(strings.TrimSpace(iface.HardwareAddr.String()))
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		items = append(items, value)
	}

	sort.Strings(items)
	return items
}
