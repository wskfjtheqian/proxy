package proxy

import (
	"io"
	"net"
	"sync"
	"webrtc_proxy/src/transfer"
)

type TCPProxy struct {
	conn      net.Conn
	proxyId   uint64
	target    string
	writeLock sync.Mutex
	transfer  *transfer.Transfer
}

func NewTCPProxy(proxyId uint64, conn net.Conn, target string, transfer *transfer.Transfer) *TCPProxy {
	ret := &TCPProxy{
		proxyId:  proxyId,
		conn:     conn,
		target:   target,
		transfer: transfer,
	}
	return ret
}

func (p *TCPProxy) Close() error {
	if p.conn != nil {
		err := p.conn.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *TCPProxy) Write(bytes []byte) (int, error) {
	p.writeLock.Lock()
	defer p.writeLock.Unlock()
	return p.conn.Write(bytes)
}

func (p *TCPProxy) ReadWriteData() {
	go func() {
		defer func() {
			_ = p.conn.Close()
			_ = p.transfer.Close(p.proxyId)
		}()
		for {
			buf := make([]byte, 1024*10)
			n, err := p.conn.Read(buf)
			if err != nil && err != io.EOF {
				return
			}
			err = p.transfer.Write(p.proxyId, buf[:n])
			if err != nil {
				return
			}
		}
	}()
}

func (p *TCPProxy) Open() error {
	return p.transfer.Open(p.proxyId, p.target)
}
