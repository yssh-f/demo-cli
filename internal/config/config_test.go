package config

import "testing"

func TestParsePorts(t *testing.T) {
	ports, err := ParsePorts("80,443,1000-1002")
	if err != nil {
		t.Fatal(err)
	}
	for _, port := range []int{80, 443, 1000, 1001, 1002} {
		if !ports.Contains(port) {
			t.Fatalf("expected port %d to match", port)
		}
	}
	if ports.Contains(999) {
		t.Fatal("did not expect port 999 to match")
	}
}

func TestParsePortsRejectsInvalidRange(t *testing.T) {
	if _, err := ParsePorts("100-10"); err == nil {
		t.Fatal("expected invalid range error")
	}
}
