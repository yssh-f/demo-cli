package config

import (
	"flag"
	"fmt"
	"io"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	CIDR    string
	IPNet   *net.IPNet
	Ports   PortSet
	Timeout time.Duration
	Format  string
	Mock    string
	Verbose bool
}

type PortSet struct {
	ranges []PortRange
}

type PortRange struct {
	Start int
	End   int
}

func ParseArgs(args []string, stderr io.Writer) (Config, error) {
	var cfg Config
	var ports string
	var timeout string

	fs := flag.NewFlagSet("mdnsmap", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&cfg.CIDR, "cidr", "", "target IPv4 CIDR, for example 192.168.1.0/24")
	fs.StringVar(&cfg.CIDR, "i", "", "target IPv4 CIDR")
	fs.StringVar(&ports, "ports", "", "ports, for example 80,443,1-1024")
	fs.StringVar(&ports, "p", "", "ports")
	fs.StringVar(&timeout, "timeout", "5s", "mDNS discovery timeout")
	fs.StringVar(&timeout, "t", "5s", "mDNS discovery timeout")
	fs.StringVar(&cfg.Format, "format", "text", "output format: text or json")
	fs.StringVar(&cfg.Format, "f", "text", "output format: text or json")
	fs.StringVar(&cfg.Mock, "mock", "", "read raw mDNS records from a mock JSON file")
	fs.BoolVar(&cfg.Verbose, "verbose", false, "print verbose diagnostics to stderr")

	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}

	return Validate(cfg, ports, timeout)
}

func Validate(cfg Config, portsExpr, timeoutExpr string) (Config, error) {
	if strings.TrimSpace(cfg.CIDR) == "" {
		return Config{}, fmt.Errorf("cidr is required")
	}
	ip, ipNet, err := net.ParseCIDR(strings.TrimSpace(cfg.CIDR))
	if err != nil || ip.To4() == nil {
		return Config{}, fmt.Errorf("invalid IPv4 CIDR %q", cfg.CIDR)
	}
	cfg.CIDR = ipNet.String()
	cfg.IPNet = ipNet

	cfg.Ports, err = ParsePorts(portsExpr)
	if err != nil {
		return Config{}, err
	}

	cfg.Timeout, err = time.ParseDuration(strings.TrimSpace(timeoutExpr))
	if err != nil || cfg.Timeout <= 0 {
		return Config{}, fmt.Errorf("invalid timeout %q", timeoutExpr)
	}

	cfg.Format = strings.ToLower(strings.TrimSpace(cfg.Format))
	if cfg.Format == "" {
		cfg.Format = "text"
	}
	if cfg.Format != "text" && cfg.Format != "json" {
		return Config{}, fmt.Errorf("unsupported format %q: use text or json", cfg.Format)
	}

	return cfg, nil
}

func ParsePorts(input string) (PortSet, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return PortSet{}, fmt.Errorf("ports is required")
	}

	result := PortSet{}
	parts := strings.Split(input, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return PortSet{}, fmt.Errorf("invalid port expression %q", input)
		}

		if strings.Contains(part, "-") {
			bounds := strings.Split(part, "-")
			if len(bounds) != 2 {
				return PortSet{}, fmt.Errorf("invalid port range %q", part)
			}
			start, err := parsePort(bounds[0])
			if err != nil {
				return PortSet{}, err
			}
			end, err := parsePort(bounds[1])
			if err != nil {
				return PortSet{}, err
			}
			if start > end {
				return PortSet{}, fmt.Errorf("invalid port range %q: start is greater than end", part)
			}
			result.ranges = append(result.ranges, PortRange{Start: start, End: end})
			continue
		}

		port, err := parsePort(part)
		if err != nil {
			return PortSet{}, err
		}
		result.ranges = append(result.ranges, PortRange{Start: port, End: port})
	}

	return result, nil
}

func (s PortSet) Contains(port int) bool {
	for _, r := range s.ranges {
		if port >= r.Start && port <= r.End {
			return true
		}
	}
	return false
}

func (s PortSet) Ranges() []PortRange {
	out := make([]PortRange, len(s.ranges))
	copy(out, s.ranges)
	return out
}

func (s PortSet) ValuesForTest() []int {
	seen := map[int]struct{}{}
	for _, r := range s.ranges {
		for p := r.Start; p <= r.End; p++ {
			seen[p] = struct{}{}
		}
	}
	values := make([]int, 0, len(seen))
	for port := range seen {
		values = append(values, port)
	}
	sort.Ints(values)
	return values
}

func parsePort(input string) (int, error) {
	input = strings.TrimSpace(input)
	port, err := strconv.Atoi(input)
	if err != nil {
		return 0, fmt.Errorf("invalid port %q", input)
	}
	if port < 1 || port > 65535 {
		return 0, fmt.Errorf("port %d out of range 1-65535", port)
	}
	return port, nil
}
