package domain

import "fmt"

// Snapshot types for persistence adapters.
// These are plain structs with exported fields for serialization.

// ActionSnapshot is the serializable form of ActionDefinition.
type ActionSnapshot struct {
	Name             string           `json:"name"`
	Description      string           `json:"description"`
	InputContract    ContractSnapshot `json:"input_contract"`
	OutputContract   ContractSnapshot `json:"output_contract"`
	Requirements     []string         `json:"requirements"`
	EffectLevel      string           `json:"effect_level"`
	IsIdempotent     bool             `json:"is_idempotent"`
	ExecutionBinding string           `json:"execution_binding"`
}

// ContractSnapshot is the serializable form of Contract.
type ContractSnapshot struct {
	Fields []ContractFieldSnapshot `json:"fields"`
}

// ContractFieldSnapshot is the serializable form of ContractField.
type ContractFieldSnapshot struct {
	Name        string `json:"name"`
	Type        string `json:"type,omitempty"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required"`
	Example     any    `json:"example,omitempty"`
}

// CapabilitySnapshot is the serializable form of CapabilityDefinition.
type CapabilitySnapshot struct {
	Name             string           `json:"name"`
	Description      string           `json:"description"`
	InputContract    ContractSnapshot `json:"input_contract"`
	OutputContract   ContractSnapshot `json:"output_contract"`
	ExecutionBinding string           `json:"execution_binding"`
}

// PluginSnapshot is the serializable form of PluginContribution.
type PluginSnapshot struct {
	PluginID     string               `json:"plugin_id"`
	Actions      []ActionSnapshot     `json:"actions"`
	Capabilities []CapabilitySnapshot `json:"capabilities"`
	Status       string               `json:"status"`
}

// SessionSnapshot is the serializable form of ExecutionSession.
type SessionSnapshot struct {
	ID                   string                    `json:"id"`
	ActionName           string                    `json:"action_name"`
	Input                any                       `json:"input"`
	Status               string                    `json:"status"`
	RequiresApproval     bool                      `json:"requires_approval"`
	ResolvedCapabilities []string                  `json:"resolved_capabilities"`
	Evidence             []EvidenceSnapshot        `json:"evidence"`
	Result               *ResultSnapshot           `json:"result,omitempty"`
	Failure              *FailureSnapshot          `json:"failure,omitempty"`
	ApprovalDecision     *ApprovalDecisionSnapshot `json:"approval_decision,omitempty"`
}

// ApprovalDecisionSnapshot is the serializable form of ApprovalDecision.
type ApprovalDecisionSnapshot struct {
	Principal string `json:"principal"`
	Rationale string `json:"rationale,omitempty"`
}

// EvidenceSnapshot is the serializable form of EvidenceRecord.
type EvidenceSnapshot struct {
	Kind      string `json:"kind"`
	Source    string `json:"source"`
	Value     any    `json:"value"`
	Timestamp int64  `json:"timestamp,omitempty"`
}

// ResultSnapshot is the serializable form of ExecutionResult.
type ResultSnapshot struct {
	Data        any    `json:"data"`
	Summary     string `json:"summary"`
	ContentType string `json:"content_type,omitempty"`
}

// FailureSnapshot is the serializable form of FailureReason.
type FailureSnapshot struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// --- ToSnapshot methods ---

func contractToSnapshot(c Contract) ContractSnapshot {
	snap := ContractSnapshot{Fields: make([]ContractFieldSnapshot, len(c.Fields))}
	for i, f := range c.Fields {
		snap.Fields[i] = ContractFieldSnapshot(f)
	}
	return snap
}

func contractFromSnapshot(s ContractSnapshot) Contract {
	fields := make([]ContractField, len(s.Fields))
	for i, f := range s.Fields {
		fields[i] = ContractField(f)
	}
	return NewContract(fields)
}

// ToSnapshot converts an ActionDefinition to a serializable snapshot.
func (a *ActionDefinition) ToSnapshot() ActionSnapshot {
	reqs := make([]string, len(a.requirements))
	for i, r := range a.requirements {
		reqs[i] = string(r.Capability)
	}
	return ActionSnapshot{
		Name:             string(a.name),
		Description:      a.description,
		InputContract:    contractToSnapshot(a.inputContract),
		OutputContract:   contractToSnapshot(a.outputContract),
		Requirements:     reqs,
		EffectLevel:      string(a.effectProfile.Level),
		IsIdempotent:     a.idempotencyProfile.IsIdempotent,
		ExecutionBinding: string(a.executionBinding),
	}
}

// ActionFromSnapshot reconstructs an ActionDefinition from a snapshot.
func ActionFromSnapshot(s ActionSnapshot) (*ActionDefinition, error) {
	name, err := NewActionName(s.Name)
	if err != nil {
		return nil, err
	}
	reqs := make([]Requirement, len(s.Requirements))
	for i, r := range s.Requirements {
		reqs[i] = Requirement{Capability: CapabilityName(r)}
	}
	reqSet, err := NewRequirementSet(reqs...)
	if err != nil {
		return nil, err
	}
	a, err := NewActionDefinition(
		name, s.Description,
		contractFromSnapshot(s.InputContract),
		contractFromSnapshot(s.OutputContract),
		reqSet,
		EffectProfile{Level: EffectLevel(s.EffectLevel)},
		IdempotencyProfile{IsIdempotent: s.IsIdempotent},
	)
	if err != nil {
		return nil, err
	}
	if s.ExecutionBinding != "" {
		_ = a.BindExecutor(ActionExecutorRef(s.ExecutionBinding))
	}
	return a, nil
}

// ToSnapshot converts a CapabilityDefinition to a serializable snapshot.
func (c *CapabilityDefinition) ToSnapshot() CapabilitySnapshot {
	return CapabilitySnapshot{
		Name:             string(c.name),
		Description:      c.description,
		InputContract:    contractToSnapshot(c.inputContract),
		OutputContract:   contractToSnapshot(c.outputContract),
		ExecutionBinding: string(c.executionBinding),
	}
}

// CapabilityFromSnapshot reconstructs a CapabilityDefinition from a snapshot.
func CapabilityFromSnapshot(s CapabilitySnapshot) (*CapabilityDefinition, error) {
	name, err := NewCapabilityName(s.Name)
	if err != nil {
		return nil, err
	}
	c, err := NewCapabilityDefinition(
		name, s.Description,
		contractFromSnapshot(s.InputContract),
		contractFromSnapshot(s.OutputContract),
	)
	if err != nil {
		return nil, err
	}
	if s.ExecutionBinding != "" {
		_ = c.BindExecutor(CapabilityExecutorRef(s.ExecutionBinding))
	}
	return c, nil
}

// ToSnapshot converts a PluginContribution to a serializable snapshot.
func (p *PluginContribution) ToSnapshot() PluginSnapshot {
	actions := make([]ActionSnapshot, len(p.actions))
	for i, a := range p.actions {
		actions[i] = a.ToSnapshot()
	}
	caps := make([]CapabilitySnapshot, len(p.capabilities))
	for i, c := range p.capabilities {
		caps[i] = c.ToSnapshot()
	}
	return PluginSnapshot{
		PluginID:     string(p.pluginID),
		Actions:      actions,
		Capabilities: caps,
		Status:       string(p.status),
	}
}

// SessionFromSnapshot reconstructs an ExecutionSession from a serializable snapshot.
// This bypasses normal state transitions to restore persisted state.
func SessionFromSnapshot(s SessionSnapshot) (*ExecutionSession, error) {
	if s.ID == "" {
		return nil, fmt.Errorf("session snapshot has empty ID")
	}
	caps := make([]CapabilityName, len(s.ResolvedCapabilities))
	for i, c := range s.ResolvedCapabilities {
		caps[i] = CapabilityName(c)
	}
	evidence := make([]EvidenceRecord, len(s.Evidence))
	for i, e := range s.Evidence {
		evidence[i] = EvidenceRecord(e)
	}
	session := &ExecutionSession{
		id:                   ExecutionSessionID(s.ID),
		actionName:           ActionName(s.ActionName),
		input:                s.Input,
		status:               ExecutionStatus(s.Status),
		requiresApproval:     s.RequiresApproval,
		resolvedCapabilities: caps,
		evidence:             evidence,
	}
	if s.Result != nil {
		session.result = &ExecutionResult{Data: s.Result.Data, Summary: s.Result.Summary, ContentType: s.Result.ContentType}
	}
	if s.Failure != nil {
		session.failure = &FailureReason{Code: s.Failure.Code, Message: s.Failure.Message}
	}
	if s.ApprovalDecision != nil {
		session.approvalDecision = &ApprovalDecision{Principal: s.ApprovalDecision.Principal, Rationale: s.ApprovalDecision.Rationale}
	}
	return session, nil
}

// ToSnapshot converts an ExecutionSession to a serializable snapshot.
func (s *ExecutionSession) ToSnapshot() SessionSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	caps := make([]string, len(s.resolvedCapabilities))
	for i, c := range s.resolvedCapabilities {
		caps[i] = string(c)
	}
	evidence := make([]EvidenceSnapshot, len(s.evidence))
	for i, e := range s.evidence {
		evidence[i] = EvidenceSnapshot(e)
	}
	snap := SessionSnapshot{
		ID:                   string(s.id),
		ActionName:           string(s.actionName),
		Input:                s.input,
		Status:               string(s.status),
		RequiresApproval:     s.requiresApproval,
		ResolvedCapabilities: caps,
		Evidence:             evidence,
	}
	if s.result != nil {
		snap.Result = &ResultSnapshot{Data: s.result.Data, Summary: s.result.Summary, ContentType: s.result.ContentType}
	}
	if s.failure != nil {
		snap.Failure = &FailureSnapshot{Code: s.failure.Code, Message: s.failure.Message}
	}
	if s.approvalDecision != nil {
		snap.ApprovalDecision = &ApprovalDecisionSnapshot{Principal: s.approvalDecision.Principal, Rationale: s.approvalDecision.Rationale}
	}
	return snap
}
