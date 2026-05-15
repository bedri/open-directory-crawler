package envutil

import (
	"os"
	"strings"
	"sync"
)

var (
	loaded sync.Once
	vars   map[string]string
)

func load() {
	vars = make(map[string]string)
	data, err := os.ReadFile(".env")
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		vars[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
}

func Get(key, fallback string) string {
	loaded.Do(load)
	if v, ok := vars[key]; ok {
		return v
	}
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
