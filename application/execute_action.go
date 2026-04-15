package application

import (
	"context"
	"fmt"

	"github.com/felixgeelhaar/axi-go/domain"
)

// IDGenerator generates unique execution session IDs.
type IDGenerator interface {
	GenerateSessionID() domain.ExecutionSessionID
}

// ExecuteActionUseCase orchestrates action execution.
type ExecuteActionUseCase struct {
	SessionRepo      domain.SessionRepository
	ExecutionService *domain.ActionExecutionService
	IDGen            IDGenerator
}

// ExecuteActionInput is the input for the use case.
type ExecuteActionInput struct {
	ActionName domain.ActionName
	Input      any
}

// ExecuteActionOutput is the output of the use case.
type ExecuteActionOutput struct {
	SessionID domain.ExecutionSessionID
	Status    domain.ExecutionStatus
	Result    *domain.ExecutionResult
	Failure   *domain.FailureReason
	Evidence  []domain.EvidenceRecord
}

// Execute runs an action and returns the execution result.
func (uc *ExecuteActionUseCase) Execute(ctx context.Context, input ExecuteActionInput) (*ExecuteActionOutput, error) {
	sessionID := uc.IDGen.GenerateSessionID()

	session, err := domain.NewExecutionSession(sessionID, input.ActionName, input.Input)
	if err != nil {
		return nil, fmt.Errorf("failed to create execution session: %w", err)
	}

	if err := uc.ExecutionService.Execute(ctx, session); err != nil {
		// Persist the session even on error (it may have partial state).
		_ = uc.SessionRepo.Save(session)
		return nil, fmt.Errorf("execution failed: %w", err)
	}

	if err := uc.SessionRepo.Save(session); err != nil {
		return nil, fmt.Errorf("failed to persist session: %w", err)
	}

	return &ExecuteActionOutput{
		SessionID: session.ID(),
		Status:    session.Status(),
		Result:    session.Result(),
		Failure:   session.Failure(),
		Evidence:  session.Evidence(),
	}, nil
}
