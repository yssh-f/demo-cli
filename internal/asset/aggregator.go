package asset

import (
	"sort"

	"mdnsmap/internal/model"
	"mdnsmap/internal/parser"
)

func Aggregate(records []model.ParsedRecord) model.ScanResult {
	hostIPs := map[string]string{}
	hostIPv6 := map[string]string{}
	serviceHosts := map[string]string{}
	servicePorts := map[string]int{}
	ptrSet := map[string]struct{}{}

	for _, record := range records {
		switch record.RecordType {
		case "A":
			if record.RawName != "" && record.IPv4 != "" {
				hostIPs[record.RawName] = record.IPv4
			}
		case "AAAA":
			if record.RawName != "" && record.IPv6 != "" {
				hostIPv6[record.RawName] = record.IPv6
			}
		case "PTR":
			if record.RawName != "" {
				ptrSet[record.RawName] = struct{}{}
			}
		case "SRV":
			key := serviceKey(record)
			if record.Hostname != "" {
				serviceHosts[key] = record.Hostname
			}
			if record.Port > 0 {
				servicePorts[key] = record.Port
			}
		}
	}

	assets := map[string]model.ServiceAsset{}
	order := []string{}

	for _, record := range records {
		switch record.RecordType {
		case "SRV", "TXT":
			key := serviceKey(record)
			current, ok := assets[key]
			if !ok {
				current = model.ServiceAsset{}
				order = append(order, key)
			}
			mergeRecord(&current, record, hostIPs, hostIPv6, serviceHosts, servicePorts)
			assets[key] = current
		}
	}

	// Some mock or library adapters may emit already flattened records.
	for _, record := range records {
		if record.RecordType != "SERVICE" {
			continue
		}
		key := serviceKey(record)
		current, ok := assets[key]
		if !ok {
			order = append(order, key)
		}
		mergeRecord(&current, record, hostIPs, hostIPv6, serviceHosts, servicePorts)
		assets[key] = current
	}

	services := make([]model.ServiceAsset, 0, len(order))
	for _, key := range order {
		asset := assets[key]
		if asset.Service == "" && asset.Port == 0 && asset.Hostname == "" && len(asset.Banner) == 0 {
			continue
		}
		services = append(services, asset)
	}

	sort.SliceStable(services, func(i, j int) bool {
		if services[i].Hostname != services[j].Hostname {
			return services[i].Hostname < services[j].Hostname
		}
		if services[i].Port != services[j].Port {
			return services[i].Port < services[j].Port
		}
		return services[i].Service < services[j].Service
	})

	return model.ScanResult{
		Services: services,
		Answers:  model.AnswerSummary{PTR: sortedKeys(ptrSet)},
	}
}

func serviceKey(record model.ParsedRecord) string {
	name := record.RawName
	if name == "" {
		name = record.RawValue
	}
	if name == "" {
		name = record.Hostname + record.Service
	}
	return name
}

func mergeRecord(asset *model.ServiceAsset, record model.ParsedRecord, hostIPs, hostIPv6 map[string]string, serviceHosts map[string]string, servicePorts map[string]int) {
	key := serviceKey(record)
	if asset.Service == "" {
		asset.Service = record.Service
	}
	if asset.Protocol == "" {
		asset.Protocol = record.Protocol
	}
	if asset.Name == "" {
		asset.Name = record.Name
	}
	if asset.Hostname == "" {
		asset.Hostname = record.Hostname
	}
	if asset.Hostname == "" {
		asset.Hostname = serviceHosts[key]
	}
	if asset.Port == 0 {
		asset.Port = record.Port
	}
	if asset.Port == 0 {
		asset.Port = servicePorts[key]
	}
	if asset.TTL == 0 {
		asset.TTL = record.TTL
	}
	if asset.IP == "" {
		asset.IP = record.IPv4
	}
	if asset.IPv6 == "" {
		asset.IPv6 = record.IPv6
	}
	if asset.Hostname != "" {
		if asset.IP == "" {
			asset.IP = hostIPs[asset.Hostname]
		}
		if asset.IPv6 == "" {
			asset.IPv6 = hostIPv6[asset.Hostname]
		}
	}
	if asset.Service == "" {
		asset.Service, asset.Protocol = parser.ServiceAndProtocol(record.RawName)
	}
	if len(record.Banner) > 0 {
		if asset.Banner == nil {
			asset.Banner = map[string]string{}
		}
		for key, value := range record.Banner {
			asset.Banner[key] = value
		}
	}
}

func sortedKeys(set map[string]struct{}) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
