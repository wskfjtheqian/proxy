// 客户端代码
package client

import (
	"encoding/binary"
	"fmt"
	"github.com/pion/webrtc/v3"
	"math/rand"
	"net"
	"sync"
	"sync/atomic"
	"time"
	"webrtc_proxy/src/public"
)

type RPCInterface interface {
	SendOfferAndICECandidates(oic public.OfferAndICECandidates) (*public.AnswerAndICECandidates, error)
}

type RTC struct {
	iceList           []webrtc.ICECandidateInit
	iceGatheringState chan webrtc.ICEGathererState
	isClosed          atomic.Bool
	close             chan struct{}
	channelCount      uint
	channel           []*webrtc.DataChannel
	channelLock       sync.RWMutex
	proxy             map[uint32]*Proxy
	proxyLock         sync.RWMutex
	rpcInterface      RPCInterface
	id                atomic.Uint32
	openResponses     map[uint32]chan string
	openLock          sync.RWMutex
	closeResponses    map[uint32]chan string
	closeLock         sync.RWMutex
	dataRequests      map[uint32]chan string
	dataLock          sync.RWMutex
}

type RTCOption func(*RTC)

func WithChannelCount(count uint) RTCOption {
	return func(c *RTC) {
		c.channelCount = count
	}
}

func NewRTC(rpcInterface RPCInterface, options ...RTCOption) *RTC {
	ret := &RTC{
		iceGatheringState: make(chan webrtc.ICEGathererState),
		iceList:           make([]webrtc.ICECandidateInit, 0),
		close:             make(chan struct{}),
		channelCount:      15,
		channel:           make([]*webrtc.DataChannel, 0),
		channelLock:       sync.RWMutex{},
		proxy:             make(map[uint32]*Proxy),
		proxyLock:         sync.RWMutex{},
		rpcInterface:      rpcInterface,
		openResponses:     make(map[uint32]chan string),
		openLock:          sync.RWMutex{},
		closeResponses:    make(map[uint32]chan string),
		closeLock:         sync.RWMutex{},
		dataRequests:      make(map[uint32]chan string),
		dataLock:          sync.RWMutex{},
	}

	for _, option := range options {
		option(ret)
	}
	return ret
}

func (r *RTC) Run() error {
	peerConnection, err := webrtc.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{URLs: []string{"stun:stun.l.google.com:19302"}},
		},
	})
	if err != nil {
		return err
	}

	peerConnection.OnICECandidate(r.onICECandidate)
	peerConnection.OnICEGatheringStateChange(r.onICEGatheringStateChange)
	peerConnection.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		println("Connection state changed:" + state.String())
		if state == webrtc.PeerConnectionStateClosed || state == webrtc.PeerConnectionStateFailed || state == webrtc.PeerConnectionStateDisconnected {
			r.Close()
		}
	})

	for i := 0; i < int(r.channelCount); i++ {
		channel, err := peerConnection.CreateDataChannel("data"+fmt.Sprintf("%d", i+1), nil)
		if err != nil {
			return err
		}

		channel.OnMessage(r.onMessage)
		channel.OnClose(func() {
			println("Data channel closed:" + channel.Label())
			r.onRmoveChannel(channel)
		})
		channel.OnOpen(func() {
			println("Data channel opened:" + channel.Label())
			r.channelLock.Lock()
			r.channel = append(r.channel, channel)
			r.channelLock.Unlock()
		})
		channel.OnError(func(err error) {
			println("Data channel error:" + channel.Label() + err.Error())
			r.onRmoveChannel(channel)
		})
	}

	offer, err := peerConnection.CreateOffer(nil)
	if err != nil {
		return err
	}
	if err = peerConnection.SetLocalDescription(offer); err != nil {
		return err
	}

	state := <-r.iceGatheringState
	if state != webrtc.ICEGathererStateComplete {
		return fmt.Errorf("ICE gathering state is not complete: %s", state.String())
	}

	answer, err := r.rpcInterface.SendOfferAndICECandidates(public.OfferAndICECandidates{
		Offer:         offer,
		ICECandidates: r.iceList,
	})
	if err != nil {
		return err
	}

	if err = peerConnection.SetRemoteDescription(answer.Answer); err != nil {
		return err
	}

	for _, candidate := range answer.ICECandidates {
		if err = peerConnection.AddICECandidate(candidate); err != nil {
			return err
		}
	}

	<-r.close
	if err = peerConnection.Close(); err != nil {
		return err
	}
	return nil
}

func (r *RTC) onICECandidate(candidate *webrtc.ICECandidate) {
	if candidate == nil {
		return
	}
	r.iceList = append(r.iceList, candidate.ToJSON())
}

func (r *RTC) Close() {
	if r.isClosed.Swap(true) {
		return
	}

	close(r.close)
	r.dataLock.RLock()
	for _, response := range r.dataRequests {
		response <- "close"
	}
	r.dataLock.RUnlock()

	r.openLock.RLock()
	for _, response := range r.openResponses {
		response <- "close"
	}
	r.openLock.RUnlock()

	r.closeLock.RLock()
	for _, response := range r.closeResponses {
		response <- "close"
	}
	r.closeLock.RUnlock()
}

func (r *RTC) onICEGatheringStateChange(state webrtc.ICEGathererState) {
	if state == webrtc.ICEGathererStateComplete || state == webrtc.ICEGathererStateClosed {
		r.iceGatheringState <- state
	}
}

func (r *RTC) onMessage(msg webrtc.DataChannelMessage) {
	if msg.IsString {
		return
	}

	tye := binary.LittleEndian.Uint32(msg.Data)
	id := binary.LittleEndian.Uint32(msg.Data[4:])
	if tye == public.MessageTypeOpenResponse {
		r.openLock.RLock()
		response, ok := r.openResponses[id]
		r.openLock.RUnlock()
		if ok {
			response <- string(msg.Data[8:])
		}
	} else if tye == public.MessageTypeCloseResponse {
		r.closeLock.RLock()
		response, ok := r.closeResponses[id]
		r.closeLock.RUnlock()
		if ok {
			response <- string(msg.Data[8:])
		}
	} else if tye == public.MessageTypeCloseRequest {
		r.proxyLock.Lock()
		proxy, ok := r.proxy[id]
		if ok {
			delete(r.proxy, id)
		}
		r.proxyLock.Unlock()
		proxy.Close()
	} else if tye == public.MessageTypeDataRequest {
		dataId := binary.LittleEndian.Uint32(msg.Data[8:])
		r.proxyLock.RLock()
		proxy, ok := r.proxy[id]
		r.proxyLock.RUnlock()

		err := fmt.Errorf("ok")
		if !ok {
			err = fmt.Errorf("no proxy for id %d", id)
		} else {
			_, err1 := proxy.Write(msg.Data[12:])
			if err1 != nil {
				err = err1
			}
		}
		data := make([]byte, 12+len(err.Error()))
		binary.LittleEndian.PutUint32(data, public.MessageTypeDataResponse)
		binary.LittleEndian.PutUint32(data[4:], id)
		binary.LittleEndian.PutUint32(data[8:], dataId)
		copy(data[12:], err.Error())

		err = r.getChannel().Send(data)
		if err != nil {
			fmt.Println(err)
		}
	} else if tye == public.MessageTypeDataResponse {
		dataId := binary.LittleEndian.Uint32(msg.Data[8:])
		r.dataLock.RLock()
		response, ok := r.dataRequests[dataId]
		r.dataLock.RUnlock()
		if ok {
			response <- string(msg.Data[12:])
		}
	}
}

func (r *RTC) getChannel() *webrtc.DataChannel {
	r.channelLock.RLock()
	channel := r.channel[rand.Intn(len(r.channel))]
	r.channelLock.RUnlock()
	return channel
}

func (r *RTC) onWrite(id uint32, bytes []byte) error {
	dataId := r.id.Add(1)

	data := make([]byte, 12+len(bytes))
	binary.LittleEndian.PutUint32(data, public.MessageTypeDataRequest)
	binary.LittleEndian.PutUint32(data[4:], id)
	binary.LittleEndian.PutUint32(data[8:], dataId)
	copy(data[12:], bytes)

	response := make(chan string, 1)
	r.dataLock.Lock()
	r.dataRequests[dataId] = response
	r.dataLock.Unlock()

	defer func() {
		r.dataLock.Lock()
		delete(r.dataRequests, id)
		close(response)
		r.dataLock.Unlock()
	}()

	if err := r.getChannel().Send(data); err != nil {
		return err
	}

	select {
	case resp := <-response:
		if resp != "ok" {
			return fmt.Errorf("data request failed: %s", response)
		}
	case <-r.close:
		return fmt.Errorf("close requested")
	case <-time.After(time.Second * 30):
		return fmt.Errorf("timed out")
	}

	return nil
}

func (r *RTC) addConn(con net.Conn, target string) (*Proxy, error) {
	id := r.id.Add(1)

	channel := r.getChannel()
	data := make([]byte, 8+len(target))
	binary.LittleEndian.PutUint32(data, public.MessageTypeOpenRequest)
	binary.LittleEndian.PutUint32(data[4:], id)
	copy(data[8:], target)

	response := make(chan string, 1)
	r.openLock.Lock()
	r.openResponses[id] = response
	r.openLock.Unlock()

	defer func() {
		r.openLock.Lock()
		delete(r.openResponses, id)
		r.openLock.Unlock()
		defer close(response)
	}()

	if err := channel.Send(data); err != nil {
		return nil, err
	}

	select {
	case resp := <-response:
		if resp != "ok" {
			return nil, fmt.Errorf("open request failed: %s", response)
		}
	case <-r.close:
		return nil, fmt.Errorf("close requested")
	case <-time.After(time.Second * 30):
		return nil, fmt.Errorf("timed out")
	}

	proxy := NewProxy(con, id, target, r.onClose, r.onWrite)
	r.proxyLock.Lock()
	r.proxy[id] = proxy
	r.proxyLock.Unlock()

	return proxy, nil
}

func (r *RTC) onClose(id uint32) error {
	data := make([]byte, 8)
	binary.LittleEndian.PutUint32(data, public.MessageTypeCloseRequest)
	binary.LittleEndian.PutUint32(data[4:], id)

	response := make(chan string, 1)
	r.closeLock.Lock()
	r.closeResponses[id] = response
	r.closeLock.Unlock()

	defer func() {
		r.closeLock.Lock()
		delete(r.closeResponses, id)
		r.closeLock.Unlock()
		defer close(response)
	}()

	if err := r.getChannel().Send(data); err != nil {
		return err
	}

	select {
	case resp := <-r.closeResponses[id]:
		if resp != "ok" {
			return fmt.Errorf("close request failed: %s", response)
		}
	case <-r.close:
		return fmt.Errorf("close requested")
	case <-time.After(time.Second * 30):
		return fmt.Errorf("timed out")
	}

	r.proxyLock.Lock()
	delete(r.proxy, id)
	r.proxyLock.Unlock()
	return nil
}

func (r *RTC) onRmoveChannel(channel *webrtc.DataChannel) {
	r.channelLock.Lock()
	for i, c := range r.channel {
		if c == channel {
			r.channel = append(r.channel[:i], r.channel[i+1:]...)
			break
		}
	}
	r.channelLock.Unlock()
}
