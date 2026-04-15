package api

import (
	"errors"
	"fmt"

	"github.com/felixgeelhaar/axi-go/application"
	"github.com/felixgeelhaar/axi-go/domain"
)

// Request DTOs.

type ExecuteActionRequest struct {
	ActionName string         `json:"action_name"`
	Input      map[string]any `json:"input"`
	Async      bool           `json:"async,omitempty"`
}

type RegisterPluginRequest struct {
	PluginID     string          `json:"plugin_id"`
	Actions      []ActionDTO     `json:"actions"`
	Capabilities []CapabilityDTO `json:"capabilities"`
}

type ActionDTO struct {
	Name           string      `json:"name"`
	Description    string      `json:"description"`
	InputContract  ContractDTO `json:"input_contract"`
	OutputContract ContractDTO `json:"output_contract"`
	Requirements   []string    `json:"requirements"`
	EffectLevel    string      `json:"effect_level"`
	IsIdempotent   bool        `json:"is_idempotent"`
	ExecutorRef    string      `json:"executor_ref"`
}

type CapabilityDTO struct {
	Name           string      `json:"name"`
	Description    string      `json:"description"`
	InputContract  ContractDTO `json:"input_contract"`
	OutputContract ContractDTO `json:"output_contract"`
	ExecutorRef    string      `json:"executor_ref"`
}

type ContractDTO struct {
	Fields []ContractFieldDTO `json:"fields"`
}

type ContractFieldDTO struct {
	Name        string `json:"name"`
	Type        string `json:"type,omitempty"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required"`
	Example     any    `json:"example,omitempty"`
}

// Response DTOs.

type ListResponse[T any] struct {
	Items []T `json:"items"`
	Count int `json:"count"`
}

type ActionResponse struct {
	Name           string      `json:"name"`
	Description    string      `json:"description"`
	InputContract  ContractDTO `json:"input_contract"`
	OutputContract ContractDTO `json:"output_contract"`
	Requirements   []string    `json:"requirements"`
	EffectLevel    string      `json:"effect_level"`
	IsIdempotent   bool        `json:"is_idempotent"`
	IsBound        bool        `json:"is_bound"`
}

type CapabilityResponse struct {
	Name           string      `json:"name"`
	Description    string      `json:"description"`
	InputContract  ContractDTO `json:"input_contract"`
	OutputContract ContractDTO `json:"output_contract"`
	IsBound        bool        `json:"is_bound"`
}

type ExecuteActionResponse struct {
	SessionID        string              `json:"session_id"`
	Status           string              `json:"status"`
	RequiresApproval bool                `json:"requires_approval,omitempty"`
	Result           *ExecutionResultDTO `json:"result,omitempty"`
	Failure          *FailureReasonDTO   `json:"failure,omitempty"`
	Evidence         []EvidenceRecordDTO `json:"evidence"`
}

type RejectSessionRequest struct {
	Reason string `json:"reason"`
}

type ExecutionResultDTO struct {
	Data        any    `json:"data"`
	Summary     string `json:"summary"`
	ContentType string `json:"content_type,omitempty"`
}

type FailureReasonDTO struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type EvidenceRecordDTO struct {
	Kind      string `json:"kind"`
	Source    string `json:"source"`
	Value     any    `json:"value"`
	Timestamp int64  `json:"timestamp,omitempty"`
}

type SessionResponse = ExecuteActionResponse

type RegisterPluginResponse struct {
	PluginID    string `json:"plugin_id"`
	Status      string `json:"status"`
	ActionCount int    `json:"action_count"`
	CapCount    int    `json:"capability_count"`
}

type ErrorResponse struct {
	Error     string `json:"error"`
	ErrorCode string `json:"error_code,omitempty"`
}

type HealthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
}

// errorResponseFromErr creates an ErrorResponse with an error_code derived from typed domain errors.
func errorResponseFromErr(err error) ErrorResponse {
	resp := ErrorResponse{Error: err.Error()}
	var notFound *domain.ErrNotFound
	var conflict *domain.ErrConflict
	var validation *domain.ErrValidation
	switch {
	case errors.As(err, &notFound):
		resp.ErrorCode = "not_found"
	case errors.As(err, &conflict):
		resp.ErrorCode = "conflict"
	case errors.As(err, &validation):
		resp.ErrorCode = "validation_error"
	default:
		resp.ErrorCode = "internal_error"
	}
	return resp
}

// Domain conversion functions.

func contractToDTO(c domain.Contract) ContractDTO {
	dto := ContractDTO{Fields: make([]ContractFieldDTO, len(c.Fields))}
	for i, f := range c.Fields {
		dto.Fields[i] = ContractFieldDTO{
			Name:        f.Name,
			Type:        f.Type,
			Description: f.Description,
			Required:    f.Required,
			Example:     f.Example,
		}
	}
	return dto
}

func contractFromDTO(dto ContractDTO) domain.Contract {
	fields := make([]domain.ContractField, len(dto.Fields))
	for i, f := range dto.Fields {
		fields[i] = domain.ContractField{
			Name:        f.Name,
			Type:        f.Type,
			Description: f.Description,
			Required:    f.Required,
			Example:     f.Example,
		}
	}
	return domain.NewContract(fields)
}

func ActionResponseFromDomain(a *domain.ActionDefinition) ActionResponse {
	reqs := make([]string, len(a.Requirements()))
	for i, r := range a.Requirements() {
		reqs[i] = string(r.Capability)
	}
	return ActionResponse{
		Name:           string(a.Name()),
		Description:    a.Description(),
		InputContract:  contractToDTO(a.InputContract()),
		OutputContract: contractToDTO(a.OutputContract()),
		Requirements:   reqs,
		EffectLevel:    string(a.EffectProfile().Level),
		IsIdempotent:   a.IdempotencyProfile().IsIdempotent,
		IsBound:        a.IsBound(),
	}
}

func CapabilityResponseFromDomain(c *domain.CapabilityDefinition) CapabilityResponse {
	return CapabilityResponse{
		Name:           string(c.Name()),
		Description:    c.Description(),
		InputContract:  contractToDTO(c.InputContract()),
		OutputContract: contractToDTO(c.OutputContract()),
		IsBound:        c.IsBound(),
	}
}

func ExecuteActionResponseFromOutput(o *application.ExecuteActionOutput) ExecuteActionResponse {
	resp := ExecuteActionResponse{
		SessionID:        string(o.SessionID),
		Status:           string(o.Status),
		RequiresApproval: o.RequiresApproval,
		Evidence:         make([]EvidenceRecordDTO, len(o.Evidence)),
	}
	if o.Result != nil {
		resp.Result = &ExecutionResultDTO{Data: o.Result.Data, Summary: o.Result.Summary, ContentType: o.Result.ContentType}
	}
	if o.Failure != nil {
		resp.Failure = &FailureReasonDTO{Code: o.Failure.Code, Message: o.Failure.Message}
	}
	for i, e := range o.Evidence {
		resp.Evidence[i] = EvidenceRecordDTO{Kind: e.Kind, Source: e.Source, Value: e.Value, Timestamp: e.Timestamp}
	}
	return resp
}

func SessionResponseFromDomain(s *domain.ExecutionSession) SessionResponse {
	resp := SessionResponse{
		SessionID: string(s.ID()),
		Status:    string(s.Status()),
		Evidence:  make([]EvidenceRecordDTO, len(s.Evidence())),
	}
	if s.Result() != nil {
		resp.Result = &ExecutionResultDTO{Data: s.Result().Data, Summary: s.Result().Summary, ContentType: s.Result().ContentType}
	}
	if s.Failure() != nil {
		resp.Failure = &FailureReasonDTO{Code: s.Failure().Code, Message: s.Failure().Message}
	}
	for i, e := range s.Evidence() {
		resp.Evidence[i] = EvidenceRecordDTO{Kind: e.Kind, Source: e.Source, Value: e.Value, Timestamp: e.Timestamp}
	}
	return resp
}

// ToDomain converts a RegisterPluginRequest into domain objects.
func (r *RegisterPluginRequest) ToDomain() (*domain.PluginContribution, error) {
	pluginID, err := domain.NewPluginID(r.PluginID)
	if err != nil {
		return nil, err
	}

	actions := make([]*domain.ActionDefinition, len(r.Actions))
	for i, a := range r.Actions {
		name, err := domain.NewActionName(a.Name)
		if err != nil {
			return nil, fmt.Errorf("action %q: %w", a.Name, err)
		}

		var reqs domain.RequirementSet
		if len(a.Requirements) > 0 {
			reqList := make([]domain.Requirement, len(a.Requirements))
			for j, rName := range a.Requirements {
				capName, err := domain.NewCapabilityName(rName)
				if err != nil {
					return nil, fmt.Errorf("action %q requirement %q: %w", a.Name, rName, err)
				}
				reqList[j] = domain.Requirement{Capability: capName}
			}
			reqs, err = domain.NewRequirementSet(reqList...)
			if err != nil {
				return nil, fmt.Errorf("action %q: %w", a.Name, err)
			}
		}

		effectLevel := domain.EffectNone
		if a.EffectLevel != "" {
			effectLevel = domain.EffectLevel(a.EffectLevel)
			if !domain.ValidEffectLevel(effectLevel) {
				return nil, fmt.Errorf("action %q: invalid effect level %q (must be none, read-local, write-local, read-external, or write-external)", a.Name, a.EffectLevel)
			}
		}

		action, err := domain.NewActionDefinition(
			name, a.Description,
			contractFromDTO(a.InputContract), contractFromDTO(a.OutputContract),
			reqs,
			domain.EffectProfile{Level: effectLevel},
			domain.IdempotencyProfile{IsIdempotent: a.IsIdempotent},
		)
		if err != nil {
			return nil, fmt.Errorf("action %q: %w", a.Name, err)
		}
		if err := action.BindExecutor(domain.ActionExecutorRef(a.ExecutorRef)); err != nil {
			return nil, fmt.Errorf("action %q: %w", a.Name, err)
		}
		actions[i] = action
	}

	capabilities := make([]*domain.CapabilityDefinition, len(r.Capabilities))
	for i, c := range r.Capabilities {
		name, err := domain.NewCapabilityName(c.Name)
		if err != nil {
			return nil, fmt.Errorf("capability %q: %w", c.Name, err)
		}
		cap, err := domain.NewCapabilityDefinition(
			name, c.Description,
			contractFromDTO(c.InputContract), contractFromDTO(c.OutputContract),
		)
		if err != nil {
			return nil, fmt.Errorf("capability %q: %w", c.Name, err)
		}
		if err := cap.BindExecutor(domain.CapabilityExecutorRef(c.ExecutorRef)); err != nil {
			return nil, fmt.Errorf("capability %q: %w", c.Name, err)
		}
		capabilities[i] = cap
	}

	return domain.NewPluginContribution(pluginID, actions, capabilities)
}
