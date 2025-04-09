package transfer

import (
	"encoding/binary"
	"errors"
	"io"
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

type TransferProxy interface {
	OnData(data []byte) error
	WriteData(buf []byte) error
	Open(addr string) error
	Close() error
	CloseNotify() chan interface{}
}

type Transfer struct {
	proxy        TransferProxy
	id           atomic.Uint64
	responseMap  map[uint64]chan []byte
	responseLock sync.RWMutex
}

func NewTransfer(rwc io.ReadWriteCloser) *Transfer {
	return &Transfer{
		responseMap:  make(map[uint64]chan []byte),
		responseLock: sync.RWMutex{},
	}
}

func (t *Transfer) OnData(data []byte) error {
	tye := binary.LittleEndian.Uint16(data)
	id := binary.LittleEndian.Uint64(data[4:])
	event := binary.LittleEndian.Uint16(data[12:])

	if tye == TransferTypeRequest {
		switch event {
		case TransferEventOpen:
			return t.writeResponse(id, event, t.proxy.Open(string(data[14:])))
		case TransferEventClose:
			return t.writeResponse(id, event, t.proxy.Close())
		case TransferEventData:
			return t.writeResponse(id, event, t.proxy.OnData(data[14:]))
		}
	} else if tye == TransferTypeResponse {
		t.responseLock.RLock()
		ch, ok := t.responseMap[id]
		if ok {
			ch <- data[14:]
		}
		t.responseLock.RUnlock()
	}
	return nil
}

func (t *Transfer) Open(addr string) error {
	return t.writeRequest(TransferEventOpen, []byte(addr))
}

func (t *Transfer) WriteData(buf []byte) error {
	return t.writeRequest(TransferEventData, buf)
}

func (t *Transfer) Close() error {
	return t.writeRequest(TransferEventClose, nil)
}

func (t *Transfer) writeRequest(event uint16, buf []byte) error {
	id := t.id.Add(1)
	data := make([]byte, 14+len(buf))
	binary.LittleEndian.PutUint16(data, TransferTypeRequest)
	binary.LittleEndian.PutUint64(data[4:], id)
	binary.LittleEndian.PutUint16(data[12:], event)

	if len(buf) > 0 {
		copy(data[14:], buf)
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

	err := t.proxy.WriteData(data)
	if err != nil {
		return err
	}

	select {
	case response := <-t.responseMap[id]:
		if binary.LittleEndian.Uint16(response) != TransferStatusOk {
			return errors.New(string(response[2:]))
		}
	case <-t.proxy.CloseNotify():
		return errors.New("connection closed")
	case <-time.After(30 * time.Second):
		return errors.New("timeout")
	}
	return nil
}

func (t *Transfer) writeResponse(id uint64, event uint16, err error) error {
	var buf []byte
	if err != nil {
		buf = []byte(err.Error())
	}

	data := make([]byte, 14+len(buf))
	binary.LittleEndian.PutUint16(data, TransferTypeRequest)
	binary.LittleEndian.PutUint64(data[4:], id)
	binary.LittleEndian.PutUint16(data[12:], event)

	if len(buf) > 0 {
		copy(data[14:], buf)
	}

	return nil
}
