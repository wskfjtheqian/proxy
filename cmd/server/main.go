package main

import (
	"log"
	"webrtc_proxy/src/server"
)

func main() {
	err := server.NewHttp().Start()
	if err != nil {
		log.Fatal(err)
	}
}
