package app

import (
	"context"
	"log"
	"net/url"
	"os"
	osSignal "os/signal"
	"strings"
	"syscall"
	"time"
	"webrtc_proxy/src/channel"
	"webrtc_proxy/src/proxy"
	"webrtc_proxy/src/signal"
)

type App struct {
	signal  *signal.Signal
	config  *Config
	channel *channel.WebRTCChannel
	proxy   *proxy.Proxy
}

func NewApp(config *Config) *App {
	return &App{
		config: config,
	}
}

func (a *App) Run() {
	// 创建一个通道用于监听系统信号
	stop := make(chan os.Signal, 1)
	osSignal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	a.start()

	// 阻塞主goroutine，直到接收到停止信号
	<-stop
	a.stop()
}

func (a *App) stop() {
	log.Println("stop signal received")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if a.signal != nil {
		err := a.signal.Stop(ctx)
		if err != nil {
			log.Println("stop signal error:", err)
		}
	}

	if a.channel != nil {
		err := a.channel.Close()
		if err != nil {
			log.Println("close channel error:", err)
		}
	}

	if a.proxy != nil {
		err := a.proxy.Close()
		if err != nil {
			log.Println("close proxy error:", err)
		}
	}
}

func (a *App) start() {
	a.startSignal()
	a.startProxy()
}

func (a *App) startSignal() {
	if a.config.Signal == nil {
		return
	}
	log.Println("start signal server")

	options := []signal.SignalOption{
		signal.WithAddr(a.config.Signal.Addr),
	}
	if a.config.Signal.UserPass != nil {
		options = append(options, signal.WithUserPass(a.config.Signal.UserPass))
	}
	a.signal = signal.NewSignal(options...)
	a.signal.Start()
}

func (a *App) startProxy() {
	if a.config.Proxy == nil {
		return
	}
	config := a.config.Proxy

	log.Println("start proxy")

	a.proxy = proxy.NewProxy()

	if 0 == strings.Index(config.SignalAddr, "ws://") || 0 == strings.Index(config.SignalAddr, "wss://") {
		parse, err := url.Parse(config.SignalAddr)
		if err != nil {
			return
		}
		query := parse.Query()
		query.Set("username", config.Username)
		query.Set("password", config.Password)
		query.Set("deviceId", a.config.DeviceId)

		ws := signal.NewWebSocket(a.config.DeviceId)
		a.channel = channel.NewWebRTCChannel(a.config.DeviceId, a.proxy)

		a.channel.RequestSignal(func(candidates *channel.SessionAndICECandidates) (*channel.SessionAndICECandidates, error) {
			return ws.RequestSignal(config.TargetDeviceID, candidates)
		})
		ws.OnResponseSignal(func(deviceId string, candidates *channel.SessionAndICECandidates) (*channel.SessionAndICECandidates, error) {
			return a.channel.Response(candidates)
		})
		ws.OnConnect(func(deviceId string) error {
			return nil
		})
		ws.OnOpen(func() {
			log.Println("signal websocket connected " + parse.String())
			if len(config.TargetDeviceID) > 0 {
				err := ws.Connect(config.TargetDeviceID)
				if err != nil {
					log.Println(err.Error())
					return
				}
				err = a.channel.Request()
				if err != nil {
					return
				}
			}
		})
		ws.OnClose(func() {
			log.Println("signal websocket closed " + parse.String())
		})

		parse.RawQuery = query.Encode()
		err = ws.Dial(parse.String())
		if err != nil {
			log.Fatalln(err.Error() + " " + parse.String())
		}
	} else {
		log.Fatalln("unsupported signal addr:", config.SignalAddr)
	}

}
