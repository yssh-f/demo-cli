package parser

import "testing"

func TestServiceAndProtocol(t *testing.T) {
	service, protocol := ServiceAndProtocol("slw-nas._qdiscover._tcp.local")
	if service != "qdiscover" || protocol != "tcp" {
		t.Fatalf("unexpected service/protocol: %s/%s", service, protocol)
	}
}

func TestParseTXT(t *testing.T) {
	banner := ParseTXT([]string{"path=/", "flag"})
	if banner["path"] != "/" {
		t.Fatalf("unexpected path: %q", banner["path"])
	}
	if banner["flag"] != "true" {
		t.Fatalf("unexpected flag: %q", banner["flag"])
	}
}
