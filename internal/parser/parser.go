package parser

import (
	"strings"

	"mdnsmap/internal/model"
)

func ParseRecords(records []model.RawRecord) []model.ParsedRecord {
	parsed := make([]model.ParsedRecord, 0, len(records))
	for _, record := range records {
		parsed = append(parsed, ParseRecord(record))
	}
	return parsed
}

func ParseRecord(record model.RawRecord) model.ParsedRecord {
	recordType := strings.ToUpper(strings.TrimSpace(record.Type))
	service, protocol := ServiceAndProtocol(record.Name)
	if service == "" {
		service, protocol = ServiceAndProtocol(record.Value)
	}

	return model.ParsedRecord{
		RecordType: recordType,
		Service:    service,
		Protocol:   protocol,
		Instance:   InstanceName(record.Name),
		Name:       DisplayName(record.Name),
		Hostname:   normalizeHost(record.Hostname),
		Port:       record.Port,
		TTL:        record.TTL,
		IPv4:       record.IPv4,
		IPv6:       record.IPv6,
		Banner:     ParseTXT(record.TXT),
		RawName:    record.Name,
		RawValue:   record.Value,
	}
}

func ParseTXT(items []string) map[string]string {
	banner := make(map[string]string)
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		key, value, ok := strings.Cut(item, "=")
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if !ok {
			banner[key] = "true"
			continue
		}
		banner[key] = strings.TrimSpace(value)
	}
	if len(banner) == 0 {
		return nil
	}
	return banner
}

func ServiceAndProtocol(name string) (string, string) {
	parts := strings.Split(strings.TrimSpace(name), ".")
	for i := 0; i < len(parts)-1; i++ {
		if strings.HasPrefix(parts[i], "_") && (parts[i+1] == "_tcp" || parts[i+1] == "_udp") {
			return strings.TrimPrefix(parts[i], "_"), strings.TrimPrefix(parts[i+1], "_")
		}
	}
	return "", ""
}

func InstanceName(name string) string {
	parts := strings.Split(strings.TrimSpace(name), ".")
	for i, part := range parts {
		if strings.HasPrefix(part, "_") {
			if i == 0 {
				return ""
			}
			return strings.Join(parts[:i], ".")
		}
	}
	return ""
}

func DisplayName(name string) string {
	instance := InstanceName(name)
	if instance == "" {
		return ""
	}
	return strings.TrimSpace(instance)
}

func normalizeHost(host string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return ""
	}
	return strings.TrimSuffix(host, ".")
}
