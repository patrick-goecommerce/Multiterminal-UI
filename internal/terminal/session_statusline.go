package terminal

// CostSource records which source last set the session's cost, so the screen
// scrape (ScanTokens) does not clobber an authoritative statusline value.
type CostSource int

const (
	CostSourceScrape     CostSource = iota // cost derived from screen scraping
	CostSourceStatusline                   // cost from Claude's statusLine JSON
)

// SetStatuslineData records official cost/context/model from Claude's statusLine
// and marks the statusline as the authoritative cost source. Called from the
// /api/statusline HTTP handler; assignment only, no I/O under the lock.
func (s *Session) SetStatuslineData(cost float64, contextPct int, model string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.contextPct = contextPct
	s.model = model
	s.costSource = CostSourceStatusline
	s.Tokens.TotalCost = cost // single displayed cost field stays authoritative
}

// StatuslineInfo returns the statusline-sourced context%/model and the current
// cost source.
func (s *Session) StatuslineInfo() (contextPct int, model string, src CostSource) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.contextPct, s.model, s.costSource
}

// SetAgentName records the name the agent itself gives the session. An empty
// name is ignored: the status line leaves the field out until a name exists,
// which is not a reason to forget one.
func (s *Session) SetAgentName(name string) {
	if name == "" {
		return
	}
	s.mu.Lock()
	s.agentName = name
	s.mu.Unlock()
}

// AgentName returns the name the agent gave the session, empty if none yet.
func (s *Session) AgentName() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.agentName
}
