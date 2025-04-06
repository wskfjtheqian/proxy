package server

import (
	"io"
	"net"
	"sync"
	"webrtc_proxy/src/public"
)

type Proxy struct {
	address   string
	id        uint32
	conn      net.Conn
	writeLock sync.Mutex
	onClose   func(id uint32) error
	onWrite   func(id uint32, bytes []byte) error
}

func NewProxy(id uint32, address string, onClose func(id uint32) error, onWrite func(id uint32, bytes []byte) error) *Proxy {
	return &Proxy{
		id:        id,
		address:   address,
		writeLock: sync.Mutex{},
		onClose:   onClose,
		onWrite:   onWrite,
	}
}

func (p *Proxy) Close() error {
	if p.conn != nil {
		err := p.conn.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *Proxy) Open() error {
	conn, err := net.Dial("tcp", p.address)
	if err != nil {
		return err
	}
	p.conn = conn
	go func() {
		defer func() {
			_ = p.conn.Close()
			_ = p.onClose(p.id)
		}()
		for {
			buf := make([]byte, public.BufferSize)
			n, err := p.conn.Read(buf)
			if err != nil && err == io.EOF {
				return
			}
			err = p.onWrite(p.id, buf[:n])
			if err != nil {
				return
			}
		}
	}()
	return nil
}

func (p *Proxy) Write(bytes []byte) (int, error) {
	p.writeLock.Lock()
	defer p.writeLock.Unlock()
	return p.conn.Write(bytes)
}
