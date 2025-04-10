package transfer

import (
	"encoding/binary"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

const TransferTypeRequest = 0xF010
const TransferTypeResponse = 0xF020

const TransferEventOpen = 0xEA21
const TransferEventClose = 0xEA22
const TransferEventData = 0xEA23

const TransferStatusOk = 0xAE01
const TransferStatusError = 0xAE02

type Proxy interface {
	OnData(proxyId uint64, data []byte) (int, error)
	Open(proxyId uint64, addr string) error
	Close(proxyId uint64) error
}

type Transfer struct {
	proxy        Proxy
	id           atomic.Uint64
	responseMap  map[uint64]chan []byte
	responseLock sync.RWMutex
	write        func(data []byte) error
}

func NewTransfer(proxy Proxy, write func(data []byte) error) *Transfer {
	return &Transfer{
		responseMap:  make(map[uint64]chan []byte),
		responseLock: sync.RWMutex{},
		proxy:        proxy,
		write:        write,
	}
}

func (t *Transfer) OnData(data []byte) error {
	proxyId := binary.LittleEndian.Uint64(data)
	tye := binary.LittleEndian.Uint16(data[8:])
	id := binary.LittleEndian.Uint64(data[10:])
	event := binary.LittleEndian.Uint16(data[18:])

	if tye == TransferTypeRequest {
		switch event {
		case TransferEventOpen:
			return t.writeResponse(proxyId, id, event, t.proxy.Open(proxyId, string(data[20:])))
		case TransferEventClose:
			return t.writeResponse(proxyId, id, event, t.proxy.Close(proxyId))
		case TransferEventData:
			_, err := t.proxy.OnData(proxyId, data[20:])
			return t.writeResponse(proxyId, id, event, err)
		}
	} else if tye == TransferTypeResponse {
		t.responseLock.RLock()
		ch, ok := t.responseMap[id]
		if ok {
			ch <- data[20:]
		}
		t.responseLock.RUnlock()
	}
	return nil
}

func (t *Transfer) Open(proxyId uint64, addr string) error {
	return t.writeRequest(proxyId, TransferEventOpen, []byte(addr))
}

func (t *Transfer) Write(proxyId uint64, buf []byte) error {
	return t.writeRequest(proxyId, TransferEventData, buf)
}

func (t *Transfer) Close(proxyId uint64) error {
	return t.writeRequest(proxyId, TransferEventClose, nil)
}

func (t *Transfer) writeRequest(proxyId uint64, event uint16, buf []byte) error {
	id := t.id.Add(1)
	data := make([]byte, 20+len(buf))
	binary.LittleEndian.PutUint64(data, proxyId)
	binary.LittleEndian.PutUint16(data[8:], TransferTypeRequest)
	binary.LittleEndian.PutUint64(data[10:], id)
	binary.LittleEndian.PutUint16(data[18:], event)

	if len(buf) > 0 {
		copy(data[20:], buf)
	}

	t.responseLock.Lock()
	t.responseMap[id] = make(chan []byte)
	t.responseLock.Unlock()

	defer func() {
		t.responseLock.Lock()
		close(t.responseMap[id])
		delete(t.responseMap, id)
		t.responseLock.Unlock()
	}()

	err := t.write(data)
	if err != nil {
		return err
	}

	select {
	case response := <-t.responseMap[id]:
		if binary.LittleEndian.Uint16(response) != TransferStatusOk {
			return errors.New(string(response[2:]))
		}
	case <-time.After(30 * time.Second):
		return errors.New("timeout")
	}
	return nil
}

func (t *Transfer) writeResponse(proxyId uint64, id uint64, event uint16, err error) error {
	var buf []byte
	if err != nil {
		buf = []byte(err.Error())
	}

	data := make([]byte, 22+len(buf))
	binary.LittleEndian.PutUint64(data, proxyId)
	binary.LittleEndian.PutUint16(data[8:], TransferTypeResponse)
	binary.LittleEndian.PutUint64(data[10:], id)
	binary.LittleEndian.PutUint16(data[18:], event)
	if len(buf) > 0 {
		binary.LittleEndian.PutUint16(data[20:], TransferStatusError)
	} else {
		binary.LittleEndian.PutUint16(data[20:], TransferStatusOk)
	}

	if len(buf) > 0 {
		copy(data[22:], buf)
	}

	return t.write(data)
}
