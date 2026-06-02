package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"

	"github.com/openai/openai-go/v2"
)

// weatherResponse maps the JSON response from WeatherAPI.
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
			},
			"required": []string{"location"},
		},
	})
}

func (t *WeatherTool) Execute(ctx context.Context, args string) (string, error) {
	var payload struct {
		Location string `json:"location"`
	}
	if err := json.Unmarshal([]byte(args), &payload); err != nil {
		return "", fmt.Errorf("failed to parse location argument: %w", err)
	}

	apiKey := os.Getenv("WEATHER_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("WEATHER_API_KEY is not set")
	}

	// Build request URL
	endpoint := fmt.Sprintf(
		"https://api.weatherapi.com/v1/current.json?key=%s&q=%s&aqi=no",
		apiKey,
		url.QueryEscape(payload.Location),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch weather: %w", err)
	}
	defer resp.Body.Close()

	// Handle API errors
	if resp.StatusCode != http.StatusOK {
		var apiErr weatherErrorResponse
		if err := json.NewDecoder(resp.Body).Decode(&apiErr); err == nil {
			return "", fmt.Errorf("weather API error: %s", apiErr.Error.Message)
		}
		return "", fmt.Errorf("weather API returned status %d", resp.StatusCode)
	}

	var weather weatherResponse
	if err := json.NewDecoder(resp.Body).Decode(&weather); err != nil {
		return "", fmt.Errorf("failed to parse weather response: %w", err)
	}

	return fmt.Sprintf(
		"Location: %s, %s | Condition: %s | Temperature: %.1f°C | Wind: %.1f kph | Humidity: %d%%",
		weather.Location.Name,
		weather.Location.Country,
		weather.Current.Condition.Text,
		weather.Current.TempC,
		weather.Current.WindKph,
		weather.Current.Humidity,
	), nil
}
