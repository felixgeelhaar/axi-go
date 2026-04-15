package api

import (
	"net/http"
	"strings"

	"github.com/felixgeelhaar/axi-go/application"
	"github.com/felixgeelhaar/axi-go/domain"
)

func (s *Server) listActions(c *Context) {
	actions := s.actionRepo.List()
	resp := make([]ActionResponse, len(actions))
	for i, a := range actions {
		resp[i] = ActionResponseFromDomain(a)
	}
	c.JSON(http.StatusOK, resp)
}

func (s *Server) getAction(c *Context) {
	name := domain.ActionName(c.Param("name"))
	action, err := s.actionRepo.GetByName(name)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, ActionResponseFromDomain(action))
}

func (s *Server) handleExecuteAction(c *Context) {
	var req ExecuteActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request body: " + err.Error()})
		return
	}
	if req.ActionName == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "action_name is required"})
		return
	}

	input := application.ExecuteActionInput{
		ActionName: domain.ActionName(req.ActionName),
		Input:      req.Input,
	}

	output, err := s.executeAction.Execute(c.Request.Context(), input)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		} else if strings.Contains(err.Error(), "validation failed") {
			status = http.StatusBadRequest
		}
		c.JSON(status, ErrorResponse{Error: err.Error()})
		return
	}

	// 200 even on action failure — failure is a valid domain outcome.
	c.JSON(http.StatusOK, ExecuteActionResponseFromOutput(output))
}

func (s *Server) getSession(c *Context) {
	id := domain.ExecutionSessionID(c.Param("id"))
	session, err := s.sessionRepo.Get(id)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, SessionResponseFromDomain(session))
}

func (s *Server) listCapabilities(c *Context) {
	capabilities := s.capabilityRepo.List()
	resp := make([]CapabilityResponse, len(capabilities))
	for i, cap := range capabilities {
		resp[i] = CapabilityResponseFromDomain(cap)
	}
	c.JSON(http.StatusOK, resp)
}

func (s *Server) handleRegisterPlugin(c *Context) {
	var req RegisterPluginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request body: " + err.Error()})
		return
	}
	if req.PluginID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "plugin_id is required"})
		return
	}

	contribution, err := req.ToDomain()
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	if err := s.registerPlugin.Execute(contribution); err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "already registered") || strings.Contains(err.Error(), "conflicts") {
			status = http.StatusConflict
		}
		c.JSON(status, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, map[string]string{
		"plugin_id": req.PluginID,
		"status":    "active",
	})
}
