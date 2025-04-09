package app

type Signal struct {
	Addr     string            `yaml:"addr"`
	UserPass map[string]string `yaml:"user_pass"`
}

type Proxy struct {
	Username       string            `yaml:"username"`
	Password       string            `yaml:"password"`
	TargetDeviceID string            `yaml:"target_device_id"`
	TcpProxy       map[string]string `yaml:"tcp_proxy"`
	SignalAddr     string            `yaml:"signal_addr"`
}

type Config struct {
	Signal   *Signal `yaml:"signal"`
	Proxy    *Proxy  `yaml:"proxy"`
	DeviceId string  `yaml:"device_id"`
}
