package proxy

import "webrtc_proxy/src/channel"

func NewProxy() *Proxy {

	return &Proxy{}
}

type Proxy struct {
	Transfer channel.Transfer
}

func (p Proxy) OnData(data []byte) error {
	//TODO implement me
	panic("implement me")
}

func (p Proxy) Open(addr string) error {
	//TODO implement me
	panic("implement me")
}

func (p Proxy) Close() error {
	//TODO implement me
	panic("implement me")
}
