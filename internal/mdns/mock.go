package mdns

import (
	"encoding/json"
	"fmt"
	"os"

	"mdnsmap/internal/model"
)

func LoadMock(path string) ([]model.RawRecord, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read mock file: %w", err)
	}
	var records []model.RawRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, fmt.Errorf("parse mock file: %w", err)
	}
	return records, nil
}
