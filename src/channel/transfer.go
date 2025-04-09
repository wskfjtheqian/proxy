package channel

type Transfer interface {
	OnData(data []byte) error

	Open(addr string) error

	Close() error
}
