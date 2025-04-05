// 服务端代码（含信令服务）
package main

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v3"
	"log"
	"net/http"
	"time"
	"webrtc_proxy/src/public"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func main() {
	//http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
	//	conn, err := upgrader.Upgrade(w, r, nil)
	//	if err != nil {
	//		log.Print(err)
	//		return
	//	}
	//	defer conn.Close()
	//
	//	// WebRTC配置
	//	config := webrtc.Configuration{
	//		ICEServers: []webrtc.ICEServer{
	//			{URLs: []string{"stun:stun.l.google.com:19302"}},
	//		},
	//	}
	//
	//	// 创建PeerConnection
	//	peerConnection, err := webrtc.NewPeerConnection(config)
	//	if err != nil {
	//		log.Fatal(err)
	//	}
	//
	//	// 创建数据通道
	//	dataChannel, err := peerConnection.CreateDataChannel("chat", nil)
	//	if err != nil {
	//		log.Fatal(err)
	//	}
	//
	//	// 设置数据通道回调
	//	dataChannel.OnOpen(func() {
	//		fmt.Println("数据通道已打开")
	//		for range time.Tick(time.Second * 3) {
	//			dataChannel.SendText("来自服务端的消息 " + time.Now().Format(time.Stamp))
	//		}
	//	})
	//
	//	dataChannel.OnMessage(func(msg webrtc.DataChannelMessage) {
	//		fmt.Printf("收到客户端消息: %s\n", string(msg.Data))
	//	})
	//
	//	// ICE Candidate处理
	//	peerConnection.OnICECandidate(func(c *webrtc.ICECandidate) {
	//		if c == nil {
	//			return
	//		}
	//
	//		marshal, err := json.Marshal(c.ToJSON())
	//		if err != nil {
	//			log.Println(err)
	//			return
	//		}
	//		if sendErr := conn.WriteJSON(map[string]interface{}{
	//			"type":    "candidate",
	//			"payload": string(marshal),
	//		}); sendErr != nil {
	//			log.Println(sendErr)
	//		}
	//	})
	//
	//	// 创建Offer
	//	offer, err := peerConnection.CreateOffer(nil)
	//	if err != nil {
	//		log.Fatal(err)
	//	}
	//
	//	// 设置本地描述
	//	if err = peerConnection.SetLocalDescription(offer); err != nil {
	//		log.Fatal(err)
	//	}
	//
	//	marshal, err := json.Marshal(offer)
	//	if err != nil {
	//		return
	//	}
	//	// 发送Offer到客户端
	//	if err = conn.WriteJSON(map[string]interface{}{
	//		"type":    "offer",
	//		"payload": string(marshal),
	//	}); err != nil {
	//		log.Fatal(err)
	//	}
	//
	//	// 处理客户端消息
	//	for {
	//		_, msg, err := conn.ReadMessage()
	//		if err != nil {
	//			log.Println(err)
	//			return
	//		}
	//
	//		var message map[string]interface{}
	//		if err := json.Unmarshal(msg, &message); err != nil {
	//			log.Println(err)
	//			continue
	//		}
	//
	//		switch message["type"] {
	//		case "answer":
	//			// 处理Answer
	//			answer := webrtc.SessionDescription{}
	//			if err := json.Unmarshal([]byte(message["payload"].(string)), &answer); err != nil {
	//				log.Fatal(err)
	//			}
	//
	//			if err := peerConnection.SetRemoteDescription(answer); err != nil {
	//				log.Fatal(err)
	//			}
	//		case "candidate":
	//			// 处理Candidate
	//			candidate := webrtc.ICECandidateInit{}
	//			if err := json.Unmarshal([]byte(message["payload"].(string)), &candidate); err != nil {
	//				log.Fatal(err)
	//			}
	//
	//			if err := peerConnection.AddICECandidate(candidate); err != nil {
	//				log.Fatal(err)
	//			}
	//		}
	//	}
	//})
	//
	//log.Println("服务端运行在 :8080")
	//log.Fatal(http.ListenAndServe(":8080", nil))
	http.HandleFunc("/offer", offerHandler)
	err := http.ListenAndServe(":8081", nil)
	if err != nil {
		return
	}
}

func offerHandler(writer http.ResponseWriter, request *http.Request) {
	var oic public.OfferAndICECandidates
	err := json.NewDecoder(request.Body).Decode(&oic)
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		log.Println(err)
		return
	}

	s := NewServer()
	aic, err := s.OnOfferAndIceCandidate(oic)
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		log.Println(err)
		return
	}
	err = json.NewEncoder(writer).Encode(aic)
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		log.Println(err)
		return
	}
	writer.WriteHeader(http.StatusOK)
}

type Server struct {
	iceList           []webrtc.ICECandidateInit
	iceGatheringState chan webrtc.ICEGathererState

	close   chan struct{}
	channel *webrtc.DataChannel
}

func NewServer() *Server {
	return &Server{
		iceList:           []webrtc.ICECandidateInit{},
		iceGatheringState: make(chan webrtc.ICEGathererState),
		close:             make(chan struct{}),
	}
}

func (s *Server) OnOfferAndIceCandidate(oic public.OfferAndICECandidates) (*public.AnswerAndICECandidates, error) {
	peerConnection, err := webrtc.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{URLs: []string{"stun:stun.l.google.com:19302"}},
		},
	})
	if err != nil {
		return nil, err
	}

	peerConnection.OnICECandidate(s.onICECandidate)
	peerConnection.OnICEGatheringStateChange(s.onICEGatheringStateChange)

	peerConnection.OnDataChannel(func(d *webrtc.DataChannel) {
		s.channel = d
		d.OnMessage(s.onMessage)
		d.OnClose(s.onClose)
		d.OnOpen(s.onOpen)
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

	state := <-s.iceGatheringState
	if state != webrtc.ICEGathererStateComplete {
		return nil, fmt.Errorf("ICE gathering state is not complete: %s", state.String())
	}

	return &public.AnswerAndICECandidates{
		Answer:        answer,
		ICECandidates: s.iceList,
	}, nil
}

func (s *Server) onICECandidate(candidate *webrtc.ICECandidate) {
	if candidate == nil {
		return
	}
	s.iceList = append(s.iceList, candidate.ToJSON())
}

func (s *Server) onICEGatheringStateChange(state webrtc.ICEGathererState) {
	if state == webrtc.ICEGathererStateComplete || state == webrtc.ICEGathererStateClosed {
		s.iceGatheringState <- state
	}
}

func (s *Server) Close() {
	close(s.close)
}

func (s *Server) onMessage(msg webrtc.DataChannelMessage) {
	fmt.Printf("收到客户端消息: %s\n", string(msg.Data))
}

func (s *Server) onClose() {
	fmt.Println("数据通道已关闭")
}

func (s *Server) onOpen() {
	fmt.Println("数据通道已打开")
	for range time.Tick(time.Second * 3) {
		s.channel.SendText("来自服务端的消息 " + time.Now().Format(time.Stamp))
	}
}
