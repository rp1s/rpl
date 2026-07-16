package runtime

import "rpl/internal/plugins"

func EnsureRuntimeAvailable(name string, author string) error {
	return plugins.EnsureAvailable(name, author)
}
