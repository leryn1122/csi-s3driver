package driver

import "github.com/leryn1122/csi-s3/pkg/support"

const (
	DriverName = "io.github.leryn.csi.s3driver"
)

type Config struct {
	DriverName string
	Version    string
	NodeID     string
	Endpoint   string
}

func NewConfig() Config {
	return Config{
		DriverName: DriverName,
		Version:    support.Version,
	}
}
