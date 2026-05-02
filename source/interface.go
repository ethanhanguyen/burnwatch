package source

type Source interface {
	Name() string
	Events() (<-chan TokenEvent, <-chan error)
}

func Discover() []Source {
	return nil
}
