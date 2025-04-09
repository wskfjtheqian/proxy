// 服务端代码（含信令服务）
package server

import (
	"encoding/binary"
	"fmt"
	"github.com/pion/webrtc/v3"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
	"webrtc_proxy/src/public"
)

type RTC struct {
	iceList           []webrtc.ICECandidateInit
	iceGatheringState chan webrtc.ICEGathererState
	isClosed          atomic.Bool
	id                atomic.Uint32
	close             chan struct{}
	channel           []*webrtc.DataChannel
	channelLock       sync.RWMutex
	proxy             map[uint32]*Proxy
	proxyLock         sync.RWMutex
	dataRequests      map[uint32]chan string
	dataLock          sync.RWMutex
}

func NewRTC() *RTC {
	return &RTC{
		iceList:           []webrtc.ICECandidateInit{},
		iceGatheringState: make(chan webrtc.ICEGathererState),
		close:             make(chan struct{}),
		channel:           []*webrtc.DataChannel{},
		channelLock:       sync.RWMutex{},
		proxy:             make(map[uint32]*Proxy),
		proxyLock:         sync.RWMutex{},
		dataRequests:      make(map[uint32]chan string),
		dataLock:          sync.RWMutex{},
	}
}

func (r *RTC) OnOfferAndIceCandidate(oic public.OfferAndICECandidates) (*public.AnswerAndICECandidates, error) {
	peerConnection, err := webrtc.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{URLs: []string{"stun:stun.l.google.com:19302"}},
		},
	})
	if err != nil {
		return nil, err
	}

	peerConnection.OnICECandidate(r.onICECandidate)
	peerConnection.OnICEGatheringStateChange(r.onICEGatheringStateChange)
	peerConnection.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		println("Connection state changed:" + state.String())
		if state == webrtc.PeerConnectionStateClosed || state == webrtc.PeerConnectionStateFailed || state == webrtc.PeerConnectionStateDisconnected {
			r.Close()
		}
	})

	peerConnection.OnDataChannel(func(channel *webrtc.DataChannel) {
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
	})

	if err = peerConnection.SetRemoteDescription(oic.Offer); err != nil {
		return nil, err
	}
	for _, candidate := range oic.ICECandidates {
		if err = peerConnection.AddICECandidate(candidate); err != nil {
			return nil, err
		}
	}

	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		return nil, err
	}

	if err = peerConnection.SetLocalDescription(answer); err != nil {
		return nil, err
	}

	state := <-r.iceGatheringState
	if state != webrtc.ICEGathererStateComplete {
		return nil, fmt.Errorf("ICE gathering state is not complete: %r", state.String())
	}

	return &public.AnswerAndICECandidates{
		Answer:        answer,
		ICECandidates: r.iceList,
	}, nil
}

func (r *RTC) onICECandidate(candidate *webrtc.ICECandidate) {
	if candidate == nil {
		return
	}
	r.iceList = append(r.iceList, candidate.ToJSON())
}

func (r *RTC) onICEGatheringStateChange(state webrtc.ICEGathererState) {
	if state == webrtc.ICEGathererStateComplete || state == webrtc.ICEGathererStateClosed {
		r.iceGatheringState <- state
	}
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
}

func (r *RTC) onMessage(msg webrtc.DataChannelMessage) {
	if msg.IsString {
		return
	}

	typ := binary.LittleEndian.Uint32(msg.Data)
	id := binary.LittleEndian.Uint32(msg.Data[4:])

	if typ == public.MessageTypeOpenRequest {
		proxy := NewProxy(id, string(msg.Data[8:]), r.onClose, r.onWrite)
		err := proxy.Open()
		data := make([]byte, 8)
		binary.LittleEndian.PutUint32(data, public.MessageTypeOpenResponse)
		binary.LittleEndian.PutUint32(data[4:], id)

		if err != nil {
			data = append(data, []byte(err.Error())...)
		} else {
			data = append(data, []byte("ok")...)
		}
		err = r.getChannel().Send(data)
		if err != nil {
			fmt.Println(err)
		}
		r.proxyLock.Lock()
		r.proxy[id] = proxy
		r.proxyLock.Unlock()

	} else if typ == public.MessageTypeCloseResponse {
		r.proxyLock.Lock()
		proxy, ok := r.proxy[id]
		if ok {
			delete(r.proxy, id)
		}
		r.proxyLock.Unlock()
		err := proxy.Close()

		data := make([]byte, 8)
		binary.LittleEndian.PutUint32(data, public.MessageTypeCloseResponse)
		binary.LittleEndian.PutUint32(data[4:], id)

		if err != nil {
			data = append(data, []byte(err.Error())...)
		} else {
			data = append(data, []byte("ok")...)
		}
		err = r.getChannel().Send(data)
		if err != nil {
			fmt.Println(err)
		}

	} else if typ == public.MessageTypeDataRequest {
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
	} else if typ == public.MessageTypeDataResponse {
		dataId := binary.LittleEndian.Uint32(msg.Data[8:])
		r.dataLock.RLock()
		response, ok := r.dataRequests[dataId]
		r.dataLock.RUnlock()
		if ok {
			response <- string(msg.Data[12:])
		}
	}
}

func (r *RTC) onClose(id uint32) error {
	data := make([]byte, 8)
	binary.LittleEndian.PutUint32(data, public.MessageTypeCloseRequest)
	binary.LittleEndian.PutUint32(data[4:], id)

	return r.getChannel().Send(data)
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

func (r *RTC) getChannel() *webrtc.DataChannel {
	r.channelLock.RLock()
	channel := r.channel[rand.Intn(len(r.channel))]
	r.channelLock.RUnlock()
	return channel
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
