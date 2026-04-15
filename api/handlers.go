package api

import (
	"errors"
	"net/http"

	"github.com/felixgeelhaar/axi-go/application"
	"github.com/felixgeelhaar/axi-go/domain"
)

func (s *Server) healthCheck(c *Context) {
	c.JSON(http.StatusOK, HealthResponse{Status: "ok", Version: "0.1.0"})
}

func (s *Server) listActions(c *Context) {
	actions := s.actionRepo.List()
	items := make([]ActionResponse, len(actions))
	for i, a := range actions {
		items[i] = ActionResponseFromDomain(a)
	}
	c.JSON(http.StatusOK, ListResponse[ActionResponse]{Items: items, Count: len(items)})
}

func (s *Server) getAction(c *Context) {
	name, err := domain.NewActionName(c.Param("name"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponseFromErr(err))
		return
	}
	action, err := s.actionRepo.GetByName(name)
	if err != nil {
		c.JSON(domainErrorStatus(err), errorResponseFromErr(err))
		return
	}
	c.JSON(http.StatusOK, ActionResponseFromDomain(action))
}

func (s *Server) handleExecuteAction(c *Context) {
	var req ExecuteActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request body: " + err.Error(), ErrorCode: "validation_error"})
		return
	}
	if req.ActionName == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "action_name is required", ErrorCode: "validation_error"})
		return
	}

	actionName, err := domain.NewActionName(req.ActionName)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponseFromErr(err))
		return
	}

	input := application.ExecuteActionInput{
		ActionName: actionName,
		Input:      req.Input,
	}

	if req.Async {
		output, err := s.executeAction.ExecuteAsync(c.Request.Context(), input)
		if err != nil {
			c.JSON(domainErrorStatus(err), errorResponseFromErr(err))
			return
		}
		c.JSON(http.StatusAccepted, ExecuteActionResponseFromOutput(output))
		return
	}

	output, err := s.executeAction.Execute(c.Request.Context(), input)
	if err != nil {
		c.JSON(domainErrorStatus(err), errorResponseFromErr(err))
		return
	}

	c.JSON(http.StatusOK, ExecuteActionResponseFromOutput(output))
}

func (s *Server) getSession(c *Context) {
	id := domain.ExecutionSessionID(c.Param("id"))
	session, err := s.sessionRepo.Get(id)
	if err != nil {
		c.JSON(domainErrorStatus(err), errorResponseFromErr(err))
		return
	}
	c.JSON(http.StatusOK, SessionResponseFromDomain(session))
}

func (s *Server) approveSession(c *Context) {
	id := domain.ExecutionSessionID(c.Param("id"))
	output, err := s.executeAction.ApproveSession(c.Request.Context(), id)
	if err != nil {
		c.JSON(domainErrorStatus(err), errorResponseFromErr(err))
		return
	}
	c.JSON(http.StatusOK, ExecuteActionResponseFromOutput(output))
}

func (s *Server) rejectSession(c *Context) {
	id := domain.ExecutionSessionID(c.Param("id"))
	var req RejectSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		req.Reason = "rejected"
	}
	output, err := s.executeAction.RejectSession(id, req.Reason)
	if err != nil {
		c.JSON(domainErrorStatus(err), errorResponseFromErr(err))
		return
	}
	c.JSON(http.StatusOK, ExecuteActionResponseFromOutput(output))
}

func (s *Server) listCapabilities(c *Context) {
	capabilities := s.capabilityRepo.List()
	items := make([]CapabilityResponse, len(capabilities))
	for i, cap := range capabilities {
		items[i] = CapabilityResponseFromDomain(cap)
	}
	c.JSON(http.StatusOK, ListResponse[CapabilityResponse]{Items: items, Count: len(items)})
}

func (s *Server) handleRegisterPlugin(c *Context) {
	var req RegisterPluginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request body: " + err.Error(), ErrorCode: "validation_error"})
		return
	}
	if req.PluginID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "plugin_id is required", ErrorCode: "validation_error"})
		return
	}

	contribution, err := req.ToDomain()
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error(), ErrorCode: "validation_error"})
		return
	}

	if err := s.registerPlugin.Execute(contribution); err != nil {
		c.JSON(domainErrorStatus(err), errorResponseFromErr(err))
		return
	}

	c.JSON(http.StatusCreated, RegisterPluginResponse{
		PluginID:    req.PluginID,
		Status:      string(domain.ContributionActive),
		ActionCount: len(req.Actions),
		CapCount:    len(req.Capabilities),
	})
}

func (s *Server) handleDeregisterPlugin(c *Context) {
	id, err := domain.NewPluginID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponseFromErr(err))
		return
	}
	if err := s.registerPlugin.Deregister(id); err != nil {
		c.JSON(domainErrorStatus(err), errorResponseFromErr(err))
		return
	}
	c.Status(http.StatusNoContent)
}

// domainErrorStatus maps domain error types to HTTP status codes.
func domainErrorStatus(err error) int {
	var notFound *domain.ErrNotFound
	if errors.As(err, &notFound) {
		return http.StatusNotFound
	}
	var conflict *domain.ErrConflict
	if errors.As(err, &conflict) {
		return http.StatusConflict
	}
	var validation *domain.ErrValidation
	if errors.As(err, &validation) {
		return http.StatusBadRequest
	}
	return http.StatusInternalServerError
}
