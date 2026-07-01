package model

type RawRecord struct {
	Type     string   `json:"type"`
	Name     string   `json:"name"`
	Value    string   `json:"value,omitempty"`
	Hostname string   `json:"hostname,omitempty"`
	Port     int      `json:"port,omitempty"`
	TTL      uint32   `json:"ttl,omitempty"`
	IPv4     string   `json:"ipv4,omitempty"`
	IPv6     string   `json:"ipv6,omitempty"`
	TXT      []string `json:"txt,omitempty"`
}

type ParsedRecord struct {
	RecordType string
	Service    string
	Protocol   string
	Instance   string
	Name       string
	Hostname   string
	Port       int
	TTL        uint32
	IPv4       string
	IPv6       string
	Banner     map[string]string
	RawName    string
	RawValue   string
}

type ServiceAsset struct {
	IP       string            `json:"ip,omitempty"`
	IPv6     string            `json:"ipv6,omitempty"`
	Port     int               `json:"port,omitempty"`
	Protocol string            `json:"protocol,omitempty"`
	Service  string            `json:"service,omitempty"`
	Name     string            `json:"name,omitempty"`
	Hostname string            `json:"hostname,omitempty"`
	TTL      uint32            `json:"ttl,omitempty"`
	Banner   map[string]string `json:"banner,omitempty"`
}

type ScanResult struct {
	Services []ServiceAsset `json:"services"`
	Answers  AnswerSummary  `json:"answers,omitempty"`
}

type AnswerSummary struct {
	PTR []string `json:"ptr,omitempty"`
}
