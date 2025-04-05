package main

import "webrtc_proxy/src/client"

func main() {
	client.NewHttp().Start()
}
