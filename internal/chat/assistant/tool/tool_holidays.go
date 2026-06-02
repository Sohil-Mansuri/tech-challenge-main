package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	ics "github.com/arran4/golang-ical"
	"github.com/openai/openai-go/v2"
)

// HolidaysTool fetches public holidays from an iCalendar feed.
type HolidaysTool struct{}

func (t *HolidaysTool) Name() string { return "get_holidays" }


// Definition tells OpenAI about this tool and its optional filter parameters.
func (t *HolidaysTool) Definition() openai.ChatCompletionToolUnionParam {
	return openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
		Name:        "get_holidays",
		Description: openai.String("Gets local bank and public holidays. Each line is a single holiday in the format 'YYYY-MM-DD: Holiday Name'."),
		Parameters: openai.FunctionParameters{
			"type": "object",
			"properties": map[string]any{
				"before_date": map[string]string{
					"type":        "string",
					"description": "Optional date in RFC3339 format to get holidays before this date.",
				},
				"after_date": map[string]string{
					"type":        "string",
					"description": "Optional date in RFC3339 format to get holidays after this date.",
				},
				"max_count": map[string]string{
					"type":        "integer",
					"description": "Optional maximum number of holidays to return.",
				},
			},
		},
	})
}

// Execute loads and filters holidays from the calendar feed.
func (t *HolidaysTool) Execute(ctx context.Context, args string) (string, error) {
	// Parse optional filter arguments
	var payload struct {
		BeforeDate time.Time `json:"before_date,omitempty"`
		AfterDate  time.Time `json:"after_date,omitempty"`
		MaxCount   int       `json:"max_count,omitempty"`
	}
	if err := json.Unmarshal([]byte(args), &payload); err != nil {
		return "", fmt.Errorf("failed to parse arguments: %w", err)
	}

	// Use env override or default to Catalonia holidays
	link := "https://www.officeholidays.com/ics/spain/catalonia"
	if v := os.Getenv("HOLIDAY_CALENDAR_LINK"); v != "" {
		link = v
	}

	events, err := LoadCalendar(ctx, link)
	if err != nil {
		return "", fmt.Errorf("failed to load calendar: %w", err)
	}

	// Filter and format holidays
	var holidays []string
	for _, event := range events {
		if payload.MaxCount > 0 && len(holidays) >= payload.MaxCount {
			break
		}

		date, err := event.GetAllDayStartAt()
		if err != nil {
			continue
		}

		if !payload.BeforeDate.IsZero() && date.After(payload.BeforeDate) {
			continue
		}

		if !payload.AfterDate.IsZero() && date.Before(payload.AfterDate) {
			continue
		}

		holidays = append(holidays,
			date.Format(time.DateOnly)+": "+event.GetProperty(ics.ComponentPropertySummary).Value,
		)
	}

	return strings.Join(holidays, "\n"), nil
}

func LoadCalendar(ctx context.Context, link string) ([]*ics.VEvent, error) {
	slog.InfoContext(ctx, "Loading calendar", "link", link)

	cal, err := ics.ParseCalendarFromUrl(link, ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to parse calendar: %w", err)
	}

	return cal.Events(), nil
}
