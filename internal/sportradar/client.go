package sportradar

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/team/websocket-poc/internal/models"
)

const playbackURL = "https://playback.sportradar.com"

type Client struct {
	http *http.Client
}

func NewClient() *Client {
	return &Client{http: &http.Client{}}
}

func (c *Client) ListSoccerRecordings(ctx context.Context) ([]models.Recording, error) {
	query := `{"query":"query { recordingsBySport(sport: \"soccer\") { id title sport league scheduled apis { name apiType formats } } }"}`

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, playbackURL+"/graphql", bytes.NewBufferString(query))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var result struct {
		Data struct {
			RecordingsBySport []models.Recording `json:"recordingsBySport"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Data.RecordingsBySport, nil
}
