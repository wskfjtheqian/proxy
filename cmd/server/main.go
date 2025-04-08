package main

import (
	"flag"
	"gopkg.in/yaml.v3"
	"log"
	"os"
	"webrtc_proxy/src/server"
)

const VERSION = "v1.0.0"
const NAME = "proxy client"

type Config struct {
	HttpAddr string `yaml:"http_addr"`
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

	if len(cfg.HttpAddr) == 0 {
		log.Fatal("http_addr is not set in config.yml")
	}

	err = server.NewHttp(
		server.WithAddr(cfg.HttpAddr),
	).Start()
	if err != nil {
		log.Fatal(err)
	}
}
