// 客户端代码
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/pion/webrtc/v3"
	"net/http"
	"time"
	"webrtc_proxy/src/public"
)

func main() {
	//u := url.URL{Scheme: "ws", Host: "localhost:8080", Path: "/ws"}
	//conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	//if err != nil {
	//	log.Fatal(err)
	//}
	//defer conn.Close()
	//
	//// WebRTC配置
	//config := webrtc.Configuration{
	//	ICEServers: []webrtc.ICEServer{
	//		{URLs: []string{"stun:stun.l.google.com:19302"}},
	//	},
	//}
	//
	//// 创建PeerConnection
	//peerConnection, err := webrtc.NewPeerConnection(config)
	//if err != nil {
	//	log.Fatal(err)
	//}
	//
	//// ICE Candidate处理
	//peerConnection.OnICECandidate(func(c *webrtc.ICECandidate) {
	//	if c == nil {
	//		return
	//	}
	//
	//	marshal, err := json.Marshal(c.ToJSON())
	//	if err != nil {
	//		log.Println(err)
	//		return
	//	}
	//	if sendErr := conn.WriteJSON(map[string]interface{}{
	//		"type":    "candidate",
	//		"payload": string(marshal),
	//	}); sendErr != nil {
	//		log.Println(sendErr)
	//	}
	//})
	//
	//// 数据通道处理
	//peerConnection.OnDataChannel(func(d *webrtc.DataChannel) {
	//	d.OnOpen(func() {
	//		log.Println("数据通道已打开")
	//		for range time.Tick(time.Second * 3) {
	//			d.SendText("来自客户端的消息 " + time.Now().Format(time.Stamp))
	//		}
	//	})
	//
	//	d.OnMessage(func(msg webrtc.DataChannelMessage) {
	//		log.Printf("收到服务端消息: %s\n", string(msg.Data))
	//	})
	//})
	//
	//// 处理服务端消息
	//for {
	//	_, msg, err := conn.ReadMessage()
	//	if err != nil {
	//		log.Println(err)
	//		return
	//	}
	//
	//	var message map[string]interface{}
	//	if err := json.Unmarshal(msg, &message); err != nil {
	//		log.Println(err)
	//		continue
	//	}
	//
	//	switch message["type"] {
	//	case "offer":
	//		// 处理Offer
	//		offer := webrtc.SessionDescription{}
	//		if err := json.Unmarshal([]byte(message["payload"].(string)), &offer); err != nil {
	//			log.Fatal(err)
	//		}
	//
	//		if err := peerConnection.SetRemoteDescription(offer); err != nil {
	//			log.Fatal(err)
	//		}
	//
	//		// 创建Answer
	//		answer, err := peerConnection.CreateAnswer(nil)
	//		if err != nil {
	//			log.Fatal(err)
	//		}
	//
	//		// 设置本地描述
	//		if err := peerConnection.SetLocalDescription(answer); err != nil {
	//			log.Fatal(err)
	//		}
	//
	//		// 发送Answer到服务端
	//		payload, err := json.Marshal(answer)
	//		if err != nil {
	//			log.Fatal(err)
	//		}
	//
	//		if err := conn.WriteJSON(map[string]interface{}{
	//			"type":    "answer",
	//			"payload": string(payload),
	//		}); err != nil {
	//			log.Fatal(err)
	//		}
	//
	//	case "candidate":
	//		// 处理Candidate
	//		candidate := webrtc.ICECandidateInit{}
	//		if err := json.Unmarshal([]byte(message["payload"].(string)), &candidate); err != nil {
	//			log.Fatal(err)
	//		}
	//
	//		if err := peerConnection.AddICECandidate(candidate); err != nil {
	//			log.Fatal(err)
	//		}
	//	}
	//}

	NewClient().Run()
}

type Client struct {
	iceList           []webrtc.ICECandidateInit
	iceGatheringState chan webrtc.ICEGathererState

	close   chan struct{}
	channel *webrtc.DataChannel
}

func NewClient() *Client {
	return &Client{
		iceGatheringState: make(chan webrtc.ICEGathererState),
		iceList:           make([]webrtc.ICECandidateInit, 0),
		close:             make(chan struct{}),
	}
}

func (c *Client) Run() error {
	peerConnection, err := webrtc.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{URLs: []string{"stun:stun.l.google.com:19302"}},
		},
	})
	if err != nil {
		return err
	}

	peerConnection.OnICECandidate(c.onICECandidate)
	peerConnection.OnICEGatheringStateChange(c.onICEGatheringStateChange)

	c.channel, err = peerConnection.CreateDataChannel("data1", nil)
	if err != nil {
		return err
	}

	c.channel.OnOpen(c.onOpen)
	c.channel.OnMessage(c.onMessage)
	c.channel.OnClose(c.onClose)

	offer, err := peerConnection.CreateOffer(nil)
	if err != nil {
		return err
	}
	if err = peerConnection.SetLocalDescription(offer); err != nil {
		return err
	}

	state := <-c.iceGatheringState
	if state != webrtc.ICEGathererStateComplete {
		return fmt.Errorf("ICE gathering state is not complete: %s", state.String())
	}

	answer, err := c.sendOfferAndICECandidates(public.OfferAndICECandidates{
		Offer:         offer,
		ICECandidates: c.iceList,
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

	<-c.close
	if err = peerConnection.Close(); err != nil {
		return err
	}
	return nil
}

func (c *Client) onICECandidate(candidate *webrtc.ICECandidate) {
	if candidate == nil {
		return
	}
	c.iceList = append(c.iceList, candidate.ToJSON())
}

func (c *Client) Close() {
	close(c.close)
}

func (c *Client) onICEGatheringStateChange(state webrtc.ICEGathererState) {
	if state == webrtc.ICEGathererStateComplete || state == webrtc.ICEGathererStateClosed {
		c.iceGatheringState <- state
	}
}

func (c *Client) sendOfferAndICECandidates(oic public.OfferAndICECandidates) (*public.AnswerAndICECandidates, error) {
	body := bytes.NewBuffer(nil)
	jsonEncoder := json.NewEncoder(body)
	if err := jsonEncoder.Encode(oic); err != nil {
		return nil, err
	}
	resp, err := http.Post("https://kpohss--8081.ap-guangzhou.cloudstudio.work/offer", "application/json", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var answer public.AnswerAndICECandidates
	if err := json.NewDecoder(resp.Body).Decode(&answer); err != nil {
		return nil, err
	}
	return &answer, nil
}

func (c *Client) onMessage(msg webrtc.DataChannelMessage) {
	fmt.Printf("收到服务端消息: %s\n", string(msg.Data))
}

func (c *Client) onClose() {
	fmt.Println("数据通道已关闭")
}

func (c *Client) onOpen() {
	fmt.Println("数据通道已打开")
	for range time.Tick(time.Second * 3) {
		c.channel.SendText("来自客户端的消息 " + time.Now().Format(time.Stamp))
	}
}
