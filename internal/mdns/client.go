package mdns

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"math/rand"
	"net"
	"strings"
	"time"

	"mdnsmap/internal/model"
)

const (
	mdnsAddr = "224.0.0.251:5353"

	typeA    uint16 = 1
	typePTR  uint16 = 12
	typeTXT  uint16 = 16
	typeAAAA uint16 = 28
	typeSRV  uint16 = 33
	classIN  uint16 = 1
)

var defaultServices = []string{
	"_services._dns-sd._udp.local",
	"_http._tcp.local",
	"_smb._tcp.local",
	"_workstation._tcp.local",
	"_device-info._tcp.local",
	"_afpovertcp._tcp.local",
	"_qdiscover._tcp.local",
}

type Client struct {
	Timeout time.Duration
}

func (c Client) Discover(ctx context.Context) ([]model.RawRecord, error) {
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	conn, err := net.ListenPacket("udp4", ":0")
	if err != nil {
		return nil, fmt.Errorf("open udp socket: %w", err)
	}
	defer conn.Close()

	dst, err := net.ResolveUDPAddr("udp4", mdnsAddr)
	if err != nil {
		return nil, err
	}

	for _, service := range defaultServices {
		query, err := buildQuery(service, typePTR)
		if err != nil {
			return nil, err
		}
		if _, err := conn.WriteTo(query, dst); err != nil {
			return nil, fmt.Errorf("send mdns query: %w", err)
		}
	}

	deadline := time.Now().Add(timeout)
	_ = conn.SetDeadline(deadline)
	records := []model.RawRecord{}
	buf := make([]byte, 9000)

	for {
		select {
		case <-ctx.Done():
			return records, ctx.Err()
		default:
		}

		n, _, err := conn.ReadFrom(buf)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				return records, nil
			}
			return records, fmt.Errorf("read mdns response: %w", err)
		}
		parsed, err := parseMessage(buf[:n])
		if err == nil {
			records = append(records, parsed...)
		}
	}
}

func buildQuery(name string, qtype uint16) ([]byte, error) {
	var buf bytes.Buffer
	_ = binary.Write(&buf, binary.BigEndian, uint16(rand.Intn(65535)))
	_ = binary.Write(&buf, binary.BigEndian, uint16(0))
	_ = binary.Write(&buf, binary.BigEndian, uint16(1))
	_ = binary.Write(&buf, binary.BigEndian, uint16(0))
	_ = binary.Write(&buf, binary.BigEndian, uint16(0))
	_ = binary.Write(&buf, binary.BigEndian, uint16(0))
	if err := writeName(&buf, name); err != nil {
		return nil, err
	}
	_ = binary.Write(&buf, binary.BigEndian, qtype)
	_ = binary.Write(&buf, binary.BigEndian, classIN)
	return buf.Bytes(), nil
}

func writeName(buf *bytes.Buffer, name string) error {
	name = strings.TrimSuffix(name, ".")
	for _, label := range strings.Split(name, ".") {
		if len(label) > 63 {
			return fmt.Errorf("dns label too long: %s", label)
		}
		buf.WriteByte(byte(len(label)))
		buf.WriteString(label)
	}
	buf.WriteByte(0)
	return nil
}

func parseMessage(data []byte) ([]model.RawRecord, error) {
	if len(data) < 12 {
		return nil, fmt.Errorf("short dns message")
	}
	qd := int(binary.BigEndian.Uint16(data[4:6]))
	an := int(binary.BigEndian.Uint16(data[6:8]))
	ns := int(binary.BigEndian.Uint16(data[8:10]))
	ar := int(binary.BigEndian.Uint16(data[10:12]))

	offset := 12
	for i := 0; i < qd; i++ {
		_, next, err := readName(data, offset)
		if err != nil {
			return nil, err
		}
		offset = next + 4
		if offset > len(data) {
			return nil, fmt.Errorf("invalid question")
		}
	}

	total := an + ns + ar
	records := make([]model.RawRecord, 0, total)
	for i := 0; i < total; i++ {
		record, next, err := parseResource(data, offset)
		if err != nil {
			return records, err
		}
		offset = next
		if record.Type != "" {
			records = append(records, record)
		}
	}
	return records, nil
}

func parseResource(data []byte, offset int) (model.RawRecord, int, error) {
	name, next, err := readName(data, offset)
	if err != nil {
		return model.RawRecord{}, offset, err
	}
	if next+10 > len(data) {
		return model.RawRecord{}, offset, fmt.Errorf("short resource header")
	}

	typ := binary.BigEndian.Uint16(data[next : next+2])
	ttl := binary.BigEndian.Uint32(data[next+4 : next+8])
	rdLen := int(binary.BigEndian.Uint16(data[next+8 : next+10]))
	rdata := next + 10
	end := rdata + rdLen
	if end > len(data) {
		return model.RawRecord{}, offset, fmt.Errorf("short rdata")
	}

	record := model.RawRecord{Name: name, TTL: ttl}
	switch typ {
	case typePTR:
		value, _, err := readName(data, rdata)
		if err != nil {
			return model.RawRecord{}, offset, err
		}
		record.Type = "PTR"
		record.Value = value
	case typeSRV:
		if rdLen < 6 {
			return model.RawRecord{}, offset, fmt.Errorf("short srv")
		}
		record.Type = "SRV"
		record.Port = int(binary.BigEndian.Uint16(data[rdata+4 : rdata+6]))
		host, _, err := readName(data, rdata+6)
		if err != nil {
			return model.RawRecord{}, offset, err
		}
		record.Hostname = strings.TrimSuffix(host, ".")
	case typeTXT:
		record.Type = "TXT"
		record.TXT = parseTXTBytes(data[rdata:end])
	case typeA:
		if rdLen == net.IPv4len {
			record.Type = "A"
			record.IPv4 = net.IP(data[rdata:end]).String()
		}
	case typeAAAA:
		if rdLen == net.IPv6len {
			record.Type = "AAAA"
			record.IPv6 = net.IP(data[rdata:end]).String()
		}
	}

	return record, end, nil
}

func readName(data []byte, offset int) (string, int, error) {
	labels := []string{}
	original := offset
	next := offset
	jumped := false
	seen := map[int]struct{}{}

	for {
		if offset >= len(data) {
			return "", original, fmt.Errorf("dns name out of range")
		}
		length := int(data[offset])
		if length == 0 {
			offset++
			if !jumped {
				next = offset
			}
			break
		}
		if length&0xC0 == 0xC0 {
			if offset+1 >= len(data) {
				return "", original, fmt.Errorf("short dns pointer")
			}
			ptr := int(binary.BigEndian.Uint16(data[offset:offset+2]) & 0x3FFF)
			if _, ok := seen[ptr]; ok {
				return "", original, fmt.Errorf("dns pointer loop")
			}
			seen[ptr] = struct{}{}
			if !jumped {
				next = offset + 2
			}
			offset = ptr
			jumped = true
			continue
		}
		if length&0xC0 != 0 {
			return "", original, fmt.Errorf("unsupported dns label")
		}
		offset++
		if offset+length > len(data) {
			return "", original, fmt.Errorf("dns label out of range")
		}
		labels = append(labels, string(data[offset:offset+length]))
		offset += length
		if !jumped {
			next = offset
		}
	}

	return strings.Join(labels, "."), next, nil
}

func parseTXTBytes(data []byte) []string {
	items := []string{}
	for len(data) > 0 {
		length := int(data[0])
		data = data[1:]
		if length > len(data) {
			break
		}
		items = append(items, string(data[:length]))
		data = data[length:]
	}
	return items
}
