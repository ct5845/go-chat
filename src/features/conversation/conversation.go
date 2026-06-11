package conversation

import (
	"cmp"
	"crypto/rand"
	"ct-go-chat/src/infrastructure/agent"
	"ct-go-chat/src/infrastructure/agent/bedrock"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

type Conversation struct {
	ID        string           `json:"id"`
	Title     string           `json:"title"`
	Created   time.Time        `json:"created"`
	Updated   time.Time        `json:"updated"`
	Totals    Totals           `json:"totals"`
	Deleted   bool             `json:"deleted,omitempty"`
	Exchanges []agent.Exchange `json:"exchanges"`
}

type Summary struct {
	ID      string    `json:"id"`
	Title   string    `json:"title"`
	Updated time.Time `json:"updated"`
}

type Store struct {
	dir     string
	modelID string
}

func NewStore(dir, modelID string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("conversation store: mkdir %s: %w", dir, err)
	}
	return &Store{dir: dir, modelID: modelID}, nil
}

func (s *Store) Load(id string) (*Conversation, error) {
	data, err := os.ReadFile(filepath.Join(s.dir, id+".json"))
	if err != nil {
		return nil, fmt.Errorf("conversation store: load %s: %w", id, err)
	}
	var c Conversation
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("conversation store: unmarshal %s: %w", id, err)
	}
	return &c, nil
}

func (s *Store) Save(c *Conversation) error {
	if c.Title == "" && len(c.Exchanges) > 0 {
		c.Title = c.Exchanges[0].Request
		if len(c.Title) > 80 {
			c.Title = c.Title[:80] + "…"
		}
	}
	c.Totals = computeTotals(c.Exchanges, s.modelID)
	c.Updated = time.Now()
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("conversation store: marshal %s: %w", c.ID, err)
	}
	if err := os.WriteFile(filepath.Join(s.dir, c.ID+".json"), data, 0o644); err != nil {
		return fmt.Errorf("conversation store: write %s: %w", c.ID, err)
	}
	return nil
}

func (s *Store) Delete(id string) error {
	c, err := s.Load(id)
	if err != nil {
		return err
	}
	c.Deleted = true
	return s.Save(c)
}

func (s *Store) List() ([]Summary, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("conversation store: list: %w", err)
	}
	var summaries []Summary
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.dir, e.Name()))
		if err != nil {
			continue
		}
		var c Conversation
		if err := json.Unmarshal(data, &c); err != nil {
			continue
		}
		if c.Deleted {
			continue
		}
		summaries = append(summaries, Summary{
			ID:      c.ID,
			Title:   c.Title,
			Updated: c.Updated,
		})
	}
	slices.SortFunc(summaries, func(a, b Summary) int {
		return cmp.Compare(b.Updated.UnixNano(), a.Updated.UnixNano())
	})
	return summaries, nil
}

type Totals struct {
	InputTokens              int     `json:"input_tokens"`
	OutputTokens             int     `json:"output_tokens"`
	CacheCreationInputTokens int     `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int     `json:"cache_read_input_tokens"`
	CostUSD                  float64 `json:"cost_usd"`
	ContextWindow            int     `json:"context_window"`
	ExchangeCount            int     `json:"exchange_count"`
	AvgResponseMs            int64   `json:"avg_response_ms"`
	LastInputTokens          int     `json:"last_input_tokens"`
	ContextUsedTokens        int     `json:"context_used_tokens"`
}

func computeTotals(exchanges []agent.Exchange, modelID string) Totals {
	t := Totals{ContextWindow: bedrock.ContextWindow(modelID)}
	var totalResponseMs int64
	var responseCount int
	for _, ex := range exchanges {
		t.InputTokens += ex.Usage.InputTokens
		t.OutputTokens += ex.Usage.OutputTokens
		t.CacheCreationInputTokens += ex.Usage.CacheCreationInputTokens
		t.CacheReadInputTokens += ex.Usage.CacheReadInputTokens
		t.CostUSD += ex.Usage.CostUSD
		t.ExchangeCount++
		if ex.Timing.TTLBMs > 0 {
			totalResponseMs += ex.Timing.TTLBMs
			responseCount++
		}
		// Context pressure is set by the most recent round: what it carried
		// is what the next request will carry again, plus its output.
		if len(ex.Rounds) > 0 {
			last := ex.Rounds[len(ex.Rounds)-1].Usage
			t.LastInputTokens = last.InputTokens
			t.ContextUsedTokens = last.InputTokens + last.CacheCreationInputTokens +
				last.CacheReadInputTokens + last.OutputTokens
		}
	}
	if responseCount > 0 {
		t.AvgResponseMs = totalResponseMs / int64(responseCount)
	}
	return t
}

func NewID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}
