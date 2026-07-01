package main

import (
	"context"
	"fmt"
	"os"

	"mdnsmap/internal/asset"
	"mdnsmap/internal/config"
	"mdnsmap/internal/filter"
	"mdnsmap/internal/mdns"
	"mdnsmap/internal/model"
	"mdnsmap/internal/output"
	"mdnsmap/internal/parser"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	cfg, err := config.ParseArgs(args, os.Stderr)
	if err != nil {
		return err
	}

	var rawRecords []model.RawRecord
	if cfg.Mock != "" {
		records, err := mdns.LoadMock(cfg.Mock)
		if err != nil {
			return err
		}
		rawRecords = records
	} else {
		client := mdns.Client{Timeout: cfg.Timeout}
		records, err := client.Discover(context.Background())
		if err != nil {
			return err
		}
		rawRecords = records
	}

	parsed := parser.ParseRecords(rawRecords)
	result := asset.Aggregate(parsed)
	result = filter.Apply(result, cfg)
	return output.Write(os.Stdout, result, cfg.Format)
}
