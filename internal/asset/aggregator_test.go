package asset

import (
	"testing"

	"mdnsmap/internal/model"
	"mdnsmap/internal/parser"
)

func TestAggregate(t *testing.T) {
	raw := []model.RawRecord{
		{Type: "PTR", Name: "_http._tcp.local", Value: "slw-nas._http._tcp.local", TTL: 10},
		{Type: "SRV", Name: "slw-nas._http._tcp.local", Hostname: "slw-nas.local", Port: 5000, TTL: 10},
		{Type: "TXT", Name: "slw-nas._http._tcp.local", TXT: []string{"path=/"}, TTL: 10},
		{Type: "A", Name: "slw-nas.local", IPv4: "192.168.1.20", TTL: 10},
	}

	result := Aggregate(parser.ParseRecords(raw))
	if len(result.Services) != 1 {
		t.Fatalf("expected one service, got %d", len(result.Services))
	}
	service := result.Services[0]
	if service.IP != "192.168.1.20" || service.Port != 5000 || service.Banner["path"] != "/" {
		t.Fatalf("unexpected service: %+v", service)
	}
	if len(result.Answers.PTR) != 1 || result.Answers.PTR[0] != "_http._tcp.local" {
		t.Fatalf("unexpected ptr answers: %+v", result.Answers.PTR)
	}
}
