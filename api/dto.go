package api

import (
	"fmt"

	"github.com/felixgeelhaar/axi-go/application"
	"github.com/felixgeelhaar/axi-go/domain"
)

// Request DTOs.

type ExecuteActionRequest struct {
	ActionName string         `json:"action_name"`
	Input      map[string]any `json:"input"`
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
	Name     string `json:"name"`
	Required bool   `json:"required"`
}

// Response DTOs.

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
	SessionID string              `json:"session_id"`
	Status    string              `json:"status"`
	Result    *ExecutionResultDTO `json:"result,omitempty"`
	Failure   *FailureReasonDTO   `json:"failure,omitempty"`
	Evidence  []EvidenceRecordDTO `json:"evidence"`
}

type ExecutionResultDTO struct {
	Data    any    `json:"data"`
	Summary string `json:"summary"`
}

type FailureReasonDTO struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type EvidenceRecordDTO struct {
	Kind   string `json:"kind"`
	Source string `json:"source"`
	Value  any    `json:"value"`
}

type SessionResponse = ExecuteActionResponse

type ErrorResponse struct {
	Error string `json:"error"`
}

// Domain conversion functions.

func contractToDTO(c domain.Contract) ContractDTO {
	dto := ContractDTO{Fields: make([]ContractFieldDTO, len(c.Fields))}
	for i, f := range c.Fields {
		dto.Fields[i] = ContractFieldDTO{Name: f.Name, Required: f.Required}
	}
	return dto
}

func contractFromDTO(dto ContractDTO) domain.Contract {
	fields := make([]domain.ContractField, len(dto.Fields))
	for i, f := range dto.Fields {
		fields[i] = domain.ContractField{Name: f.Name, Required: f.Required}
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
		SessionID: string(o.SessionID),
		Status:    string(o.Status),
		Evidence:  make([]EvidenceRecordDTO, len(o.Evidence)),
	}
	if o.Result != nil {
		resp.Result = &ExecutionResultDTO{Data: o.Result.Data, Summary: o.Result.Summary}
	}
	if o.Failure != nil {
		resp.Failure = &FailureReasonDTO{Code: o.Failure.Code, Message: o.Failure.Message}
	}
	for i, e := range o.Evidence {
		resp.Evidence[i] = EvidenceRecordDTO{Kind: e.Kind, Source: e.Source, Value: e.Value}
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
		resp.Result = &ExecutionResultDTO{Data: s.Result().Data, Summary: s.Result().Summary}
	}
	if s.Failure() != nil {
		resp.Failure = &FailureReasonDTO{Code: s.Failure().Code, Message: s.Failure().Message}
	}
	for i, e := range s.Evidence() {
		resp.Evidence[i] = EvidenceRecordDTO{Kind: e.Kind, Source: e.Source, Value: e.Value}
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
			return nil, fmt.Errorf("action %d: %w", i, err)
		}

		var reqs domain.RequirementSet
		if len(a.Requirements) > 0 {
			reqList := make([]domain.Requirement, len(a.Requirements))
			for j, rName := range a.Requirements {
				capName, err := domain.NewCapabilityName(rName)
				if err != nil {
					return nil, fmt.Errorf("action %d requirement %d: %w", i, j, err)
				}
				reqList[j] = domain.Requirement{Capability: capName}
			}
			reqs, err = domain.NewRequirementSet(reqList...)
			if err != nil {
				return nil, fmt.Errorf("action %d: %w", i, err)
			}
		}

		effectLevel := domain.EffectNone
		if a.EffectLevel != "" {
			effectLevel = domain.EffectLevel(a.EffectLevel)
		}

		action, err := domain.NewActionDefinition(
			name, a.Description,
			contractFromDTO(a.InputContract), contractFromDTO(a.OutputContract),
			reqs,
			domain.EffectProfile{Level: effectLevel},
			domain.IdempotencyProfile{IsIdempotent: a.IsIdempotent},
		)
		if err != nil {
			return nil, fmt.Errorf("action %d: %w", i, err)
		}
		if err := action.BindExecutor(domain.ActionExecutorRef(a.ExecutorRef)); err != nil {
			return nil, fmt.Errorf("action %d: %w", i, err)
		}
		actions[i] = action
	}

	capabilities := make([]*domain.CapabilityDefinition, len(r.Capabilities))
	for i, c := range r.Capabilities {
		name, err := domain.NewCapabilityName(c.Name)
		if err != nil {
			return nil, fmt.Errorf("capability %d: %w", i, err)
		}
		cap, err := domain.NewCapabilityDefinition(
			name, c.Description,
			contractFromDTO(c.InputContract), contractFromDTO(c.OutputContract),
		)
		if err != nil {
			return nil, fmt.Errorf("capability %d: %w", i, err)
		}
		if err := cap.BindExecutor(domain.CapabilityExecutorRef(c.ExecutorRef)); err != nil {
			return nil, fmt.Errorf("capability %d: %w", i, err)
		}
		capabilities[i] = cap
	}

	return domain.NewPluginContribution(pluginID, actions, capabilities)
}
