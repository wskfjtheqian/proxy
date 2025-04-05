package main

import (
	"log"
	"webrtc_proxy/src/client"
)

func main() {
	http := client.NewHttp(
		client.WithUrl("https://kpohss--9090.ap-guangzhou.cloudstudio.work"))

	rtc := client.NewRTC(http)

	listener := client.NewListener(rtc, map[string]string{
		":8000": "127.0.0.1:8000",
	})
	err := listener.Start()
	if err != nil {
		return
	}

	err = rtc.Run()
	if err != nil {
		log.Fatal(err)
		return
	}
}
