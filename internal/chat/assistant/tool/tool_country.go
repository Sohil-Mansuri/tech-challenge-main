package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/openai/openai-go/v2"
)

type CountryResponse struct {
	Name struct {
		Common   string `json:"common"`
		Official string `json:"official"`
	} `json:"name"`
	Capital    []string          `json:"capital"`
	Population int               `json:"population"`
	Region     string            `json:"region"`
	Subregion  string            `json:"subregion"`
	Languages  map[string]string `json:"languages"`
	Currencies map[string]struct {
		Name   string `json:"name"`
		Symbol string `json:"symbol"`
	} `json:"currencies"`
	Timezones []string `json:"timezones"`
	Borders   []string `json:"borders"`
	Flags     struct {
		Alt string `json:"alt"`
	} `json:"flags"`
}

type CountryTool struct{}

func (t *CountryTool) Name() string { return "get_country_info" }

func (t *CountryTool) Definition() openai.ChatCompletionToolUnionParam {
	return openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
		Name:        "get_country_info",
		Description: openai.String("Get information about a country including capital, currency, languages, population, timezones and bordering countries. Useful for travel questions about visas, money, language and geography."),
		Parameters: openai.FunctionParameters{
			"type": "object",
			"properties": map[string]any{
				"country": map[string]string{
					"type":        "string",
					"description": "Country name in English e.g. Spain, India, Japan, United States",
				},
			},
			"required": []string{"country"},
		},
	})
}

func (t *CountryTool) Execute(ctx context.Context, args string) (string, error) {
	// parse country name from args
	var payload struct {
		Country string `json:"country"`
	}
	if err := json.Unmarshal([]byte(args), &payload); err != nil {
		return "", fmt.Errorf("failed to parse arguments: %w", err)
	}

	if payload.Country == "" {
		return "", fmt.Errorf("country name is required")
	}

	endpoint := fmt.Sprintf(
		"https://restcountries.com/v3.1/name/%s?fields=name,capital,population,region,subregion,languages,currencies,timezones,borders",
		url.QueryEscape(payload.Country),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch country info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf("country %q not found", payload.Country)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("country API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var countries []CountryResponse
	if err := json.Unmarshal(body, &countries); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if len(countries) == 0 {
		return "", fmt.Errorf("no results found for country %q", payload.Country)
	}

	return formatCountry(countries[0]), nil
}

func formatCountry(c CountryResponse) string {
	var lines []string

	lines = append(lines, fmt.Sprintf("Country: %s (%s)", c.Name.Common, c.Name.Official))

	// capital
	if len(c.Capital) > 0 {
		lines = append(lines, fmt.Sprintf("Capital: %s", strings.Join(c.Capital, ", ")))
	}

	// region
	if c.Subregion != "" {
		lines = append(lines, fmt.Sprintf("Region: %s, %s", c.Subregion, c.Region))
	} else {
		lines = append(lines, fmt.Sprintf("Region: %s", c.Region))
	}

	// population
	lines = append(lines, fmt.Sprintf("Population: %s", formatPopulation(c.Population)))

	// languages
	if len(c.Languages) > 0 {
		langs := make([]string, 0, len(c.Languages))
		for _, v := range c.Languages {
			langs = append(langs, v)
		}
		lines = append(lines, fmt.Sprintf("Languages: %s", strings.Join(langs, ", ")))
	}

	// currencies
	if len(c.Currencies) > 0 {
		var currencies []string
		for code, cur := range c.Currencies {
			currencies = append(currencies, fmt.Sprintf("%s (%s, %s)", cur.Name, cur.Symbol, code))
		}
		lines = append(lines, fmt.Sprintf("Currency: %s", strings.Join(currencies, ", ")))
	}

	// timezones
	if len(c.Timezones) > 0 {
		lines = append(lines, fmt.Sprintf("Timezones: %s", strings.Join(c.Timezones, ", ")))
	}

	// bordering countries
	if len(c.Borders) > 0 {
		lines = append(lines, fmt.Sprintf("Borders: %s", strings.Join(c.Borders, ", ")))
	} else {
		lines = append(lines, "Borders: None (island or isolated country)")
	}

	return strings.Join(lines, "\n")
}

func formatPopulation(n int) string {
	s := fmt.Sprintf("%d", n)
	result := ""
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result += ","
		}
		result += string(c)
	}
	return result
}
