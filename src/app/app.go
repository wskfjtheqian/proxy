package app

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	osSignal "os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"webrtc_proxy/src/channel"
	"webrtc_proxy/src/proxy"
	"webrtc_proxy/src/signal"
	"webrtc_proxy/src/transfer"
)

func NewApp(config *Config) *App {
	return &App{
		config:    config,
		proxyMap:  make(map[uint64]*proxy.TCPProxy),
		proxyLock: sync.RWMutex{},
	}
}

type App struct {
	signal    *signal.Signal
	config    *Config
	channel   *channel.WebRTCChannel
	transfer  *transfer.Transfer
	proxyMap  map[uint64]*proxy.TCPProxy
	proxyLock sync.RWMutex
	proxyId   atomic.Uint64
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

	a.transfer = transfer.NewTransfer(a, func(data []byte) error {
		if a.channel == nil {
			return fmt.Errorf("No data channel available")
		}
		return a.channel.Send(data)
	})

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
		a.channel = channel.NewWebRTCChannel(a.config.DeviceId, a.transfer)

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

	if len(config.TcpProxy) > 0 {
		for src, dest := range config.TcpProxy {
			go func() {
				err := a.listenTcpProxy(src, dest)
				if err != nil {
					log.Println(err.Error())
				}
			}()
		}
	}
}

func (a *App) OnData(proxyId uint64, data []byte) (int, error) {
	a.proxyLock.RLock()
	p, ok := a.proxyMap[proxyId]
	a.proxyLock.RUnlock()

	if ok {
		return p.Write(data)
	}
	return 0, fmt.Errorf("proxy not found")
}

func (a *App) Open(proxyId uint64, addr string) error {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}
	p := proxy.NewTCPProxy(proxyId, conn, addr, a.transfer)
	p.ReadWriteData()

	a.proxyLock.Lock()
	a.proxyMap[proxyId] = p
	a.proxyLock.Unlock()
	return nil
}

func (a *App) Close(proxyId uint64) error {
	a.proxyLock.Lock()
	defer a.proxyLock.Unlock()
	if p, ok := a.proxyMap[proxyId]; ok {
		delete(a.proxyMap, proxyId)
		return p.Close()
	}
	return fmt.Errorf("proxy not found")
}

func (a *App) listenTcpProxy(src string, dest string) error {
	listen, err := net.Listen("tcp", src)
	if err != nil {
		return err
	}
	for {
		conn, err := listen.Accept()
		if err != nil {
			return err
		}

		proxyId := a.proxyId.Add(1)
		p := proxy.NewTCPProxy(proxyId, conn, dest, a.transfer)
		err = p.Open()
		if err != nil {
			return conn.Close()
		}
		p.ReadWriteData()

		a.proxyLock.Lock()
		a.proxyMap[proxyId] = p
		a.proxyLock.Unlock()
	}
}
