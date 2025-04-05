package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/pion/webrtc/v3"
)

func main() {
	// 配置 PeerConnection
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}

	// 创建 PeerConnection
	peerConnection, err := webrtc.NewPeerConnection(config)
	if err != nil {
		panic(err)
	}

	// 创建数据通道
	dataChannel, err := peerConnection.CreateDataChannel("data", nil)
	if err != nil {
		panic(err)
	}

	// 设置数据通道回调
	dataChannel.OnOpen(func() {
		fmt.Printf("Data channel '%s'-'%d' 已打开\n", dataChannel.Label(), dataChannel.ID())
		// 发送测试消息
		if err := dataChannel.SendText("Hello from Go!"); err != nil {
			fmt.Println("发送失败:", err)
		}
	})

	dataChannel.OnMessage(func(msg webrtc.DataChannelMessage) {
		fmt.Printf("收到消息: %s\n", string(msg.Data))
	})

	// 处理 ICE 候选
	peerConnection.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c == nil {
			return
		}
		// 这里需要将候选发送给远端（实际应用中需要通过信令服务器）
		payload, err := json.Marshal(c.ToJSON())
		if err != nil {
			fmt.Println("候选序列化失败:", err)
			return
		}
		fmt.Println("本地候选:", string(payload))
	})

	// 创建 Offer
	offer, err := peerConnection.CreateOffer(nil)
	if err != nil {
		panic(err)
	}

	// 设置本地描述
	err = peerConnection.SetLocalDescription(offer)
	if err != nil {
		panic(err)
	}

	// 这里应该通过信令服务器交换 SDP 和候选
	// 此处简化流程，直接处理远程 Answer
	// 在实际应用中，这里需要接收远程的 Answer 和候选

	// 保持程序运行
	select {
	case <-time.After(30 * time.Second):
		fmt.Println("程序结束")
	}
}
