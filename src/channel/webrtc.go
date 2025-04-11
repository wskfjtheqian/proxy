package channel

import (
	"fmt"
	"github.com/pion/webrtc/v3"
	"log"
	"math/rand"
	"strconv"
	"sync"
)

type Transfer interface {
	OnData(data []byte) error
}

type SendSignal func(*SessionAndICECandidates) (*SessionAndICECandidates, error)

type SessionAndICECandidates struct {
	Session       webrtc.SessionDescription `json:"session"`
	ICECandidates []webrtc.ICECandidateInit `json:"ice_candidates"`
}

type WebRTCChannel struct {
	peerConnection    *webrtc.PeerConnection
	channelCount      uint
	iceList           []webrtc.ICECandidateInit
	iceGatheringState chan webrtc.ICEGathererState
	sendSignal        SendSignal
	channel           []*webrtc.DataChannel
	channelLock       sync.RWMutex
	transfer          Transfer
	deviceId          string
}

func NewWebRTCChannel(deviceId string, transfer Transfer) *WebRTCChannel {
	ret := &WebRTCChannel{
		deviceId:          deviceId,
		channelCount:      15,
		iceList:           make([]webrtc.ICECandidateInit, 0),
		iceGatheringState: make(chan webrtc.ICEGathererState),
		channel:           make([]*webrtc.DataChannel, 0),
		channelLock:       sync.RWMutex{},
		transfer:          transfer,
	}

	return ret
}

func (w *WebRTCChannel) Request() error {
	err := w.newPeerConnection()
	if err != nil {
		return err
	}

	for i := uint(0); i < w.channelCount; i++ {
		channel, err := w.peerConnection.CreateDataChannel("dataChannel"+strconv.Itoa(int(i)), nil)
		if err != nil {
			return err
		}
		w.onDataChannel(channel)
	}

	offer, err := w.peerConnection.CreateOffer(nil)
	if err != nil {
		return err
	}
	if err = w.peerConnection.SetLocalDescription(offer); err != nil {
		return err
	}

	state := <-w.iceGatheringState
	if state != webrtc.ICEGathererStateComplete {
		return fmt.Errorf("ICE gathering state is not complete: %s", state.String())
	}

	answer, err := w.sendSignal(&SessionAndICECandidates{
		Session:       offer,
		ICECandidates: w.iceList,
	})
	if err != nil {
		return err
	}

	if err = w.peerConnection.SetRemoteDescription(answer.Session); err != nil {
		return err
	}

	for _, candidate := range answer.ICECandidates {
		if err = w.peerConnection.AddICECandidate(candidate); err != nil {
			return err
		}
	}
	return nil
}

func (w *WebRTCChannel) Response(candidates *SessionAndICECandidates) (*SessionAndICECandidates, error) {
	err := w.newPeerConnection()
	if err != nil {
		return nil, err
	}

	w.peerConnection.OnDataChannel(w.onDataChannel)
	err = w.peerConnection.SetRemoteDescription(candidates.Session)
	if err != nil {
		return nil, err
	}

	for _, candidate := range candidates.ICECandidates {
		err = w.peerConnection.AddICECandidate(candidate)
		if err != nil {
			return nil, err
		}
	}

	answer, err := w.peerConnection.CreateAnswer(nil)
	if err != nil {
		return nil, err
	}

	if err = w.peerConnection.SetLocalDescription(answer); err != nil {
		return nil, err
	}

	state := <-w.iceGatheringState
	if state != webrtc.ICEGathererStateComplete {
		return nil, fmt.Errorf("ICE gathering state is not complete: %r", state.String())
	}

	return &SessionAndICECandidates{
		Session:       answer,
		ICECandidates: w.iceList,
	}, nil
}

func (w *WebRTCChannel) newPeerConnection() error {
	peerConnection, err := webrtc.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{URLs: []string{"stun:stun.l.google.com:19302"}},
		},
	})
	if err != nil {
		return err
	}

	peerConnection.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate == nil {
			return
		}
		log.Println("ICE candidate received:", candidate.ToJSON())
		w.iceList = append(w.iceList, candidate.ToJSON())
	})
	peerConnection.OnICEGatheringStateChange(func(state webrtc.ICEGathererState) {
		log.Println("ICE gathering state changed:" + state.String())
		if state == webrtc.ICEGathererStateComplete || state == webrtc.ICEGathererStateClosed {
			w.iceGatheringState <- state
		}
	})
	peerConnection.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		log.Println("Connection state changed:" + state.String())
		if state == webrtc.PeerConnectionStateClosed || state == webrtc.PeerConnectionStateFailed || state == webrtc.PeerConnectionStateDisconnected {

		}
	})

	w.peerConnection = peerConnection
	return nil
}

func (w *WebRTCChannel) Close() error {
	if w.peerConnection != nil && w.peerConnection.ConnectionState() != webrtc.PeerConnectionStateClosed {
		w.peerConnection = nil
		err := w.peerConnection.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func (w *WebRTCChannel) onDataChannel(channel *webrtc.DataChannel) {
	channel.OnMessage(func(msg webrtc.DataChannelMessage) {
		err := w.transfer.OnData(msg.Data)
		if err != nil {
			log.Println("Error on data channel:", err)
		}
	})
	channel.OnClose(func() {
		log.Println("Data channel closed:" + channel.Label())
		w.onRemoveChannel(channel)
	})
	channel.OnOpen(func() {
		log.Println("Data channel opened:" + channel.Label())
		w.channelLock.Lock()
		w.channel = append(w.channel, channel)
		w.channelLock.Unlock()
	})
	channel.OnError(func(err error) {
		log.Println("Data channel error:" + channel.Label() + err.Error())
		w.onRemoveChannel(channel)
	})
}

func (w *WebRTCChannel) onRemoveChannel(channel *webrtc.DataChannel) {
	w.channelLock.Lock()
	for i, c := range w.channel {
		if c == channel {
			w.channel = append(w.channel[:i], w.channel[i+1:]...)
			break
		}
	}
	w.channelLock.Unlock()
}

func (w *WebRTCChannel) RequestSignal(signal func(candidates *SessionAndICECandidates) (*SessionAndICECandidates, error)) {
	w.sendSignal = signal
}

func (w *WebRTCChannel) Send(data []byte) error {
	w.channelLock.Lock()
	defer w.channelLock.Unlock()
	if len(w.channel) == 0 {
		return fmt.Errorf("No data channel available")
	}

	return w.channel[rand.Intn(len(w.channel))].Send(data)
}
