package utils

import (
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/logger"
	"net"
)

const (
	DefaultHost = "127.0.0.1"
)

func GetLocalIP() string {
	netInterfaces, err := net.Interfaces()
	if err != nil {
		logger.GetLogger().Errorf("failed to get local host", err)
		return DefaultHost
	}

	for _, netInterface := range netInterfaces {
		// 过滤回环地址
		if netInterface.Flags&net.FlagLoopback == net.FlagLoopback {
			continue
		}

		address, err := netInterface.Addrs()
		if err != nil {
			logger.GetLogger().Errorf("failed to get local host", err)
			return DefaultHost
		}

		var ip net.IP
		for _, addr := range address {
			switch addr := addr.(type) {
			case *net.IPNet:
				ip = addr.IP.To4()
			case *net.IPAddr:
				ip = addr.IP.To4()
			}
			if ip == nil {
				continue
			}

			return ip.String()
		}

	}

	return DefaultHost

}
