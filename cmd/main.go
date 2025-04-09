package main

import (
	"flag"
	"gopkg.in/yaml.v3"
	"log"
	"os"
	"webrtc_proxy/src/app"
)

const VERSION = "v1.0.0"
const NAME = "proxy application"

func main() {
	log.Printf("%s version: %s", NAME, VERSION)

	configFile := flag.String("config", "config.yml", "path to config file")
	flag.Parse()

	file, err := os.Open(*configFile)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	cfg := app.Config{}
	err = yaml.NewDecoder(file).Decode(&cfg)
	if err != nil {
		log.Fatal(err)
	}

	app.NewApp(
		&cfg,
	).Run()
}
