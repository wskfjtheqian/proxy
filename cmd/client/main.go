package main

import (
	"flag"
	"gopkg.in/yaml.v3"
	"log"
	"os"
	"webrtc_proxy/src/client"
)

const VERSION = "v1.0.0"
const NAME = "proxy client"

type Config struct {
	ChannelCount uint              `yaml:"channel_count"`
	ProxyAddr    map[string]string `yaml:"proxy_addr"`
	BaseUrl      string            `yaml:"base_url"`
}

func main() {
	log.Printf("%s version: %s", NAME, VERSION)

	configFile := flag.String("config", "config.yml", "path to config file")
	flag.Parse()

	file, err := os.Open(*configFile)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	cfg := Config{}
	err = yaml.NewDecoder(file).Decode(&cfg)
	if err != nil {
		log.Fatal(err)
	}

	if len(cfg.BaseUrl) == 0 {
		log.Fatal("base_url is not set in config.yml")
	}

	http := client.NewHttp(
		client.WithBaseUrl(cfg.BaseUrl),
	)

	if cfg.ChannelCount == 0 {
		cfg.ChannelCount = 15
	}
	rtc := client.NewRTC(http, client.WithChannelCount(cfg.ChannelCount))

	listener := client.NewListener(rtc, cfg.ProxyAddr)
	err = listener.Start()
	if err != nil {
		return
	}

	err = rtc.Run()
	if err != nil {
		log.Fatal(err)
		return
	}
}
