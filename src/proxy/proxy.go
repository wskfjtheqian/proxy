package proxy

import "webrtc_proxy/src/channel"

func NewProxy() *Proxy {

	return &Proxy{}
}

type Proxy struct {
	Transfer channel.Transfer
}

func (p Proxy) OnData(data []byte) error {
	return nil
}

func (p Proxy) Open(addr string) error {
	return p.Transfer.Open(addr)
}

func (p Proxy) Close() error {
	return nil
}
