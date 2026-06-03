package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/openai/openai-go/v2"
)

type weatherResponse struct {
	Location struct {
		Name    string `json:"name"`
		Country string `json:"country"`
	} `json:"location"`
	Current struct {
		TempC     float64 `json:"temp_c"`
		Condition struct {
			Text string `json:"text"`
		} `json:"condition"`
		WindKph  float64 `json:"wind_kph"`
		Humidity int     `json:"humidity"`
	} `json:"current"`
}

type ForecastResponse struct {
	Location struct {
		Name    string `json:"name"`
		Country string `json:"country"`
	} `json:"location"`
	Forecast struct {
		ForecastDay []struct {
			Date string `json:"date"`
			Day  struct {
				MaxTempC  float64 `json:"maxtemp_c"`
				MinTempC  float64 `json:"mintemp_c"`
				Condition struct {
					Text string `json:"text"`
				} `json:"condition"`
				DailyChanceOfRain int `json:"daily_chance_of_rain"`
			} `json:"day"`
		} `json:"forecastday"`
	} `json:"forecast"`
}

// weatherErrorResponse maps error responses from WeatherAPI.
type weatherErrorResponse struct {
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

// WeatherTool fetches real weather data for a given location.
type WeatherTool struct{}

func (t *WeatherTool) Name() string { return "get_weather" }

func (t *WeatherTool) Definition() openai.ChatCompletionToolUnionParam {
	return openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
		Name:        "get_weather",
		Description: openai.String("Get current weather at the given location"),
		Parameters: openai.FunctionParameters{
			"type": "object",
			"properties": map[string]any{
				"location": map[string]string{
					"type":        "string",
					"description": "City name or coordinates",
				},
				"forecast_days": map[string]string{
					"type":        "integer",
					"description": "Number of days to forecast (1-10). If not provided, returns current weather only.",
				},
			},
			"required": []string{"location"},
		},
	})
}

func (t *WeatherTool) Execute(ctx context.Context, args string) (string, error) {
	var payload struct {
		Location     string `json:"location"`
		ForecastDays int    `json:"forecast_days,omitempty"`
	}
	if err := json.Unmarshal([]byte(args), &payload); err != nil {
		return "", fmt.Errorf("failed to parse location argument: %w", err)
	}

	apiKey := os.Getenv("WEATHER_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("WEATHER_API_KEY is not set")
	}

	// if forecast_days is set → use forecast API, otherwise use current
	if payload.ForecastDays > 0 {
		return getForecast(ctx, apiKey, payload.Location, payload.ForecastDays)
	}
	return getCurrent(ctx, apiKey, payload.Location)
}

func getCurrent(ctx context.Context, apiKey, location string) (string, error) {
	endpoint := fmt.Sprintf(
		"https://api.weatherapi.com/v1/current.json?key=%s&q=%s&aqi=no",
		apiKey,
		url.QueryEscape(location),
	)

	body, err := doRequest(ctx, endpoint)
	if err != nil {
		return "", err
	}

	var resp weatherResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	return fmt.Sprintf(
		"Location: %s, %s | Condition: %s | Temperature: %.1f°C | Wind: %.1f kph | Humidity: %d%%",
		resp.Location.Name,
		resp.Location.Country,
		resp.Current.Condition.Text,
		resp.Current.TempC,
		resp.Current.WindKph,
		resp.Current.Humidity,
	), nil
}

func getForecast(ctx context.Context, apiKey, location string, days int) (string, error) {

	if days > 10 {
		days = 10
	}

	endpoint := fmt.Sprintf(
		"https://api.weatherapi.com/v1/forecast.json?key=%s&q=%s&days=%d&aqi=no",
		apiKey,
		url.QueryEscape(location),
		days,
	)

	body, err := doRequest(ctx, endpoint)
	if err != nil {
		return "", err
	}

	var resp ForecastResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	// format each day as one line
	var lines []string
	lines = append(lines, fmt.Sprintf("Location: %s, %s", resp.Location.Name, resp.Location.Country))
	for _, day := range resp.Forecast.ForecastDay {
		lines = append(lines, fmt.Sprintf(
			"%s | Condition: %s | High: %.1f°C | Low: %.1f°C | Rain chance: %d%%",
			day.Date,
			day.Day.Condition.Text,
			day.Day.MaxTempC,
			day.Day.MinTempC,
			day.Day.DailyChanceOfRain,
		))
	}

	return strings.Join(lines, "\n"), nil
}

func doRequest(ctx context.Context, endpoint string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var apiErr weatherErrorResponse
		if err := json.NewDecoder(resp.Body).Decode(&apiErr); err == nil {
			return nil, fmt.Errorf("weather API error: %s", apiErr.Error.Message)
		}
		return nil, fmt.Errorf("weather API returned status %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}
