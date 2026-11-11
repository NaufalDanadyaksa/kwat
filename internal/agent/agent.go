package agent

import (
	"context"
	"fmt"
	"os"

	"github.com/md101/kwat/internal/config"
	"github.com/md101/kwat/internal/llm"
	"github.com/md101/kwat/internal/permission"
	"github.com/md101/kwat/internal/session"
	"github.com/md101/kwat/internal/tools"
)

// Agent represents the main autonomous coding assistant instance.
type Agent struct {
	Config       *config.Config
	Provider     llm.Provider
	PermManager  *permission.Manager
	Executor     ToolExecutor
	Engine       *ReActEngine
	Session      *session.Session
	SessionStore *session.Store
}

// NewAgent initializes an Agent with specified configuration and dependencies.
func NewAgent(cfg *config.Config, permManager *permission.Manager, executor ToolExecutor, customRules string) (*Agent, error) {
	if cfg == nil {
		cfg = config.DefaultConfig()
	}

	providerName := cfg.DefaultProvider
	provCfg, ok := cfg.Providers[providerName]
	if !ok {
		provCfg = config.ProviderConfig{Model: cfg.DefaultModel}
	}
	if provCfg.Model == "" {
		provCfg.Model = cfg.DefaultModel
	}

	provider, err := CreateProvider(providerName, provCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create provider %q: %w", providerName, err)
	}

	if permManager == nil {
		permMode := permission.ParseMode(cfg.PermissionMode)
		permManager = permission.NewManager(permMode, false)
	}

	if executor == nil {
		executor = tools.DefaultRegistry("")
	}

	sysPrompt := BuildSystemPrompt(customRules, "")
	engine := NewReActEngine(provider, provCfg.Model, executor, permManager, sysPrompt)

	workDir, _ := os.Getwd()
	sessStore, _ := session.NewStore("")
	sess := session.NewSession(providerName, provCfg.Model, workDir)

	return &Agent{
		Config:       cfg,
		Provider:     provider,
		PermManager:  permManager,
		Executor:     executor,
		Engine:       engine,
		Session:      sess,
		SessionStore: sessStore,
	}, nil
}

// SwitchModel dynamically changes the active provider and model at runtime.
func (a *Agent) SwitchModel(providerName, modelName string) error {
	provCfg, ok := a.Config.Providers[providerName]
	if !ok {
		provCfg = config.ProviderConfig{Model: modelName}
	} else {
		provCfg.Model = modelName
	}

	newProvider, err := CreateProvider(providerName, provCfg)
	if err != nil {
		return err
	}

	a.Provider = newProvider
	a.Config.DefaultProvider = providerName
	a.Config.DefaultModel = modelName
	a.Engine.Provider = newProvider
	a.Engine.Model = modelName
	if a.Session != nil {
		a.Session.Provider = providerName
		a.Session.Model = modelName
		if a.SessionStore != nil {
			_ = a.SessionStore.Save(a.Session)
		}
	}
	return nil
}

// SetApprover sets the interactive callback for asking user approval.
func (a *Agent) SetApprover(approver ApproverFunc) {
	a.Engine.Approver = approver
}

// ResumeSession loads a previous session's message history into the agent.
func (a *Agent) ResumeSession(sessionID string) error {
	if a.SessionStore == nil {
		return fmt.Errorf("session store not initialized")
	}

	var sess *session.Session
	var err error
	if sessionID == "" || sessionID == "latest" {
		sess, err = a.SessionStore.GetLatest()
	} else {
		sess, err = a.SessionStore.Load(sessionID)
	}

	if err != nil {
		return fmt.Errorf("failed to resume session: %w", err)
	}

	a.Session = sess
	a.Engine.History = sess.Messages
	return nil
}

// CompactSession manually triggers token compression on the conversation history.
func (a *Agent) CompactSession() (int, int) {
	beforeTokens := session.EstimateMessagesTokens(a.Engine.History)
	a.Engine.History = session.Compact(a.Engine.History, 4)
	afterTokens := session.EstimateMessagesTokens(a.Engine.History)

	if a.Session != nil {
		a.Session.Messages = a.Engine.History
		a.Session.TotalTokens = afterTokens
		if a.SessionStore != nil {
			_ = a.SessionStore.Save(a.Session)
		}
	}

	return beforeTokens, afterTokens
}

// Ask sends a user prompt to the agent, auto-compacts context if needed, and persists the session.
func (a *Agent) Ask(ctx context.Context, prompt string, eventChan chan<- TurnEvent) (*TurnResult, error) {
	// Auto-compaction check before running turn
	if a.Config.AutoCompact && session.NeedsCompaction(a.Engine.History, a.Config.MaxContextTokens) {
		a.Engine.History = session.Compact(a.Engine.History, 4)
	}

	res, err := a.Engine.RunStep(ctx, prompt, eventChan)

	// Persist session state
	if a.Session != nil {
		a.Session.Messages = a.Engine.History
		a.Session.TotalTokens = session.EstimateMessagesTokens(a.Engine.History)
		if a.SessionStore != nil {
			_ = a.SessionStore.Save(a.Session)
		}
	}

	return res, err
}

