package filter

import (
	"net"

	"mdnsmap/internal/config"
	"mdnsmap/internal/model"
)

func Apply(result model.ScanResult, cfg config.Config) model.ScanResult {
	filtered := make([]model.ServiceAsset, 0, len(result.Services))
	for _, service := range result.Services {
		if !matchesCIDR(service, cfg.IPNet) {
			continue
		}
		if !matchesPort(service, cfg.Ports) {
			continue
		}
		filtered = append(filtered, service)
	}
	result.Services = filtered
	return result
}

func matchesCIDR(service model.ServiceAsset, ipNet *net.IPNet) bool {
	if ipNet == nil {
		return true
	}
	if service.IP == "" {
		if service.Service == "device-info" {
			return true
		}
		return service.IPv6 != ""
	}
	ip := net.ParseIP(service.IP)
	if ip == nil {
		return false
	}
	return ipNet.Contains(ip)
}

func matchesPort(service model.ServiceAsset, ports config.PortSet) bool {
	if service.Port == 0 {
		return service.Service == "device-info"
	}
	return ports.Contains(service.Port)
}
