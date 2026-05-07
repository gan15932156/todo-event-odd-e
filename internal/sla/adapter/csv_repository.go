package adapter

import (
	"context"
	"encoding/csv"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"
	"todoe/internal/sla/domain"

	"github.com/samber/mo"
)

// this adapter for create csv file with header sla-2026-05-07.csv
// this file contain time and status of task
type CSVRepository struct {
}

func NewCSVRepository() *CSVRepository {
	return &CSVRepository{}
}

func (r *CSVRepository) WriteCSV(ctx context.Context, event domain.SLA) mo.Result[struct{}] {
	slog.Info("sla: writing csv", "event", event)
	// Determine CSV file name based on current date
	dateStr := time.Now().Format("2006-01-02")
	fileName := fmt.Sprintf("sla-%s.csv", dateStr)
	filePath := filepath.Join(".", fileName)

	// Check if file exists to decide whether to write header
	_, err := os.Stat(filePath)
	fileExists := err == nil

	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return mo.Err[struct{}](err)
	}
	defer f.Close()

	writer := csv.NewWriter(f)
	defer writer.Flush()

	if !fileExists {
		// Write header
		if err := writer.Write([]string{"ID", "Time", "Event"}); err != nil {
			return mo.Err[struct{}](err)
		}
	}

	record := []string{event.ID, event.Time.Format(time.RFC3339), event.Event}
	if err := writer.Write(record); err != nil {
		return mo.Err[struct{}](err)
	}
	return mo.Ok(struct{}{})
}
