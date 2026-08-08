package commands

import "sort"

type Handler func(args []string) error

var registry = map[string]Handler{}

func Register(name string, h Handler) {
	registry[name] = h
}

func Lookup(name string) (Handler, bool) {
	h, ok := registry[name]
	return h, ok
}

func Names() []string {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
