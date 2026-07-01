package output

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"mdnsmap/internal/model"
)

func Write(w io.Writer, result model.ScanResult, format string) error {
	switch format {
	case "json":
		return writeJSON(w, result)
	default:
		return writeText(w, result)
	}
}

func writeJSON(w io.Writer, result model.ScanResult) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}

func writeText(w io.Writer, result model.ScanResult) error {
	if _, err := fmt.Fprintln(w, "services:"); err != nil {
		return err
	}
	for _, service := range result.Services {
		if service.Port > 0 {
			if _, err := fmt.Fprintf(w, "%d/%s %s:\n", service.Port, service.Protocol, service.Service); err != nil {
				return err
			}
		} else {
			if _, err := fmt.Fprintf(w, "%s:\n", service.Service); err != nil {
				return err
			}
		}
		if service.Name != "" {
			fmt.Fprintf(w, "Name=%s\n", service.Name)
		}
		if service.IP != "" {
			fmt.Fprintf(w, "IPv4=%s\n", service.IP)
		}
		if service.IPv6 != "" {
			fmt.Fprintf(w, "IPv6=%s\n", service.IPv6)
		}
		if service.Hostname != "" {
			fmt.Fprintf(w, "Hostname=%s\n", service.Hostname)
		}
		if service.TTL > 0 {
			fmt.Fprintf(w, "TTL=%d\n", service.TTL)
		}
		if len(service.Banner) > 0 {
			fmt.Fprintln(w, formatBanner(service.Banner))
		}
		fmt.Fprintln(w)
	}

	if len(result.Answers.PTR) > 0 {
		fmt.Fprintln(w, "answers:")
		fmt.Fprintln(w, "PTR:")
		for _, ptr := range result.Answers.PTR {
			fmt.Fprintln(w, ptr)
		}
	}
	return nil
}

func formatBanner(banner map[string]string) string {
	keys := make([]string, 0, len(banner))
	for key := range banner {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+banner[key])
	}
	return strings.Join(parts, ",")
}
