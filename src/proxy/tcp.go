package proxy

import "io"

type TCPProxy struct {
	srcAddr string
	dstAddr string
}

func NewTCPProxy(srcAddr, dstAddr string) *TCPProxy {
	ret := &TCPProxy{
		srcAddr: srcAddr,
		dstAddr: dstAddr,
	}
	return ret
}

func (p *TCPProxy) Read(b []byte) (n int, err error) {

	return 0, io.EOF
}

func (p *TCPProxy) Write(b []byte) (n int, err error) {
	return 0, nil
}

func (p *TCPProxy) Close() error {
	return nil
}
