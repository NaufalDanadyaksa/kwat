package session

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/md101/kwat/internal/llm"
)

// Session represents a persistent conversation and task execution session.
type Session struct {
	ID           string        `json:"id"`
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
	WorkDir      string        `json:"work_dir"`
	Provider     string        `json:"provider"`
	Model        string        `json:"model"`
	Messages     []llm.Message `json:"messages"`
	TotalTokens  int           `json:"total_tokens"`
}

// NewSession creates a new session instance.
func NewSession(provider, model, workDir string) *Session {
	now := time.Now()
	return &Session{
		ID:        generateSessionID(now),
		CreatedAt: now,
		UpdatedAt: now,
		WorkDir:   workDir,
		Provider:  provider,
		Model:     model,
		Messages:  make([]llm.Message, 0),
	}
}

func generateSessionID(t time.Time) string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s-%s", t.Format("20060102-150405"), hex.EncodeToString(b))
}

