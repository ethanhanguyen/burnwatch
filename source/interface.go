package source

import "os"

type Source interface {
	Name() string
	Events() (<-chan TokenEvent, <-chan error)
}

func Discover() []Source {
	var sources []Source

	if dbPath := defaultDBPath(); dbPath != "" {
		if _, err := os.Stat(dbPath); err == nil {
			sources = append(sources, &OpenCodeSource{dbPath: dbPath})
		}
	}

	return sources
}
