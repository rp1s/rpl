//go:build !darwin && !linux && !windows

package fingerprint

func osSpecificSources() []Source {
	return nil
}
