package snapshot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// Client communicates with the alisten-snapshot service
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new snapshot client
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Save persists a snapshot to the snapshot service
func (c *Client) Save(snap *Snapshot) error {
	data, err := json.Marshal(snap)
	if err != nil {
		return fmt.Errorf("marshal snapshot: %w", err)
	}
	resp, err := c.httpClient.Post(c.baseURL+"/snapshot/save", "application/json", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("post snapshot: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("save failed (%d): %s", resp.StatusCode, body)
	}
	return nil
}

// Load retrieves the latest snapshot from the snapshot service
func (c *Client) Load() (*Snapshot, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/snapshot/load")
	if err != nil {
		return nil, fmt.Errorf("get snapshot: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("load failed (%d): %s", resp.StatusCode, body)
	}
	var snap Snapshot
	if err := json.NewDecoder(resp.Body).Decode(&snap); err != nil {
		return nil, fmt.Errorf("decode snapshot: %w", err)
	}
	return &snap, nil
}

// Health checks if the snapshot service is reachable
func (c *Client) Health() bool {
	resp, err := c.httpClient.Get(c.baseURL + "/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// SaveWithRetry attempts to save a snapshot with retries
func (c *Client) SaveWithRetry(snap *Snapshot, retries int) error {
	var err error
	for i := 0; i < retries; i++ {
		if err = c.Save(snap); err == nil {
			return nil
		}
		log.Printf("snapshot save attempt %d/%d failed: %v", i+1, retries, err)
		time.Sleep(time.Duration(i+1) * time.Second)
	}
	return fmt.Errorf("snapshot save failed after %d retries: %w", retries, err)
}
