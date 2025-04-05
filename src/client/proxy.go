package client

import (
	"io"
	"log"
	"net"
	"sync"
)

type Listener struct {
	rtc     *RTC
	address map[string]string
}

func NewListener(rtc *RTC, address map[string]string) *Listener {
	return &Listener{
		rtc:     rtc,
		address: address,
	}
}

func (l *Listener) Listen(address string, target string) error {
	println("proxy  " + address + "->" + target)
	listen, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}
	for {
		con, err := listen.Accept()
		if err != nil {
			return err
		}

		proxy, err := l.rtc.addConn(con, target)
		if err != nil {
			log.Println(err)
			return err
		}
		err = proxy.ReadWriteData()
		if err != nil {
			log.Println(err)
		}
	}
}

func (l *Listener) Start() error {
	for addr, target := range l.address {
		go func() {
			err := l.Listen(addr, target)
			if err != nil {
				log.Println(err)
			}
		}()
	}
	return nil
}

type Proxy struct {
	conn      net.Conn
	id        uint32
	target    string
	onClose   func(id uint32) error
	onWrite   func(id uint32, bytes []byte) error
	writeLock sync.Mutex
}

func NewProxy(conn net.Conn, id uint32, target string, onClose func(id uint32) error, write func(id uint32, bytes []byte) error) *Proxy {
	return &Proxy{
		conn:      conn,
		id:        id,
		target:    target,
		onClose:   onClose,
		onWrite:   write,
		writeLock: sync.Mutex{},
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

func (p *Proxy) Write(bytes []byte) (int, error) {
	p.writeLock.Lock()
	defer p.writeLock.Unlock()
	return p.conn.Write(bytes)
}

func (p *Proxy) ReadWriteData() error {
	go func() {
		defer func() {
			_ = p.conn.Close()
			_ = p.onClose(p.id)
		}()
		for {
			buf := make([]byte, 10240)
			n, err := p.conn.Read(buf)
			if err != nil && err != io.EOF {
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
