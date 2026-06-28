package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"hatesentry/internal/auth"
	"hatesentry/internal/clients"
	apperrors "hatesentry/internal/errors"
)

// ClientHandler handles admin client application management.
type ClientHandler struct {
	service *clients.Service
}

// NewClientHandler creates a new client handler.
func NewClientHandler(service *clients.Service) *ClientHandler {
	return &ClientHandler{service: service}
}

// CreateClientRequest is the admin request body for creating an external client.
type CreateClientRequest struct {
	Name          string `json:"name" binding:"required"`
	WebhookURL    string `json:"webhook_url"`
	PolicyVersion string `json:"policy_version"`
}

// Create creates an external client and returns its raw API key once.
func (h *ClientHandler) Create(c *gin.Context) {
	claims, exists := auth.GetClaims(c)
	if !exists {
		apperrors.RespondWithError(c, apperrors.Unauthorized("User not authenticated"))
		return
	}
	if h.service == nil {
		apperrors.RespondWithError(c, apperrors.ConfigurationError("client service is not configured"))
		return
	}

	var req CreateClientRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apperrors.RespondWithError(c, apperrors.ValidationError("Invalid request body").WithDetails(err.Error()))
		return
	}

	output, err := h.service.CreateClient(c.Request.Context(), clients.CreateInput{
		UserID:        claims.UserID,
		Name:          req.Name,
		WebhookURL:    req.WebhookURL,
		PolicyVersion: req.PolicyVersion,
	})
	if err != nil {
		apperrors.Handle(c, err)
		return
	}

	c.JSON(http.StatusCreated, output)
}

// List returns external client records without raw key material.
func (h *ClientHandler) List(c *gin.Context) {
	if _, exists := auth.GetClaims(c); !exists {
		apperrors.RespondWithError(c, apperrors.Unauthorized("User not authenticated"))
		return
	}
	if h.service == nil {
		apperrors.RespondWithError(c, apperrors.ConfigurationError("client service is not configured"))
		return
	}

	output, err := h.service.ListClients(c.Request.Context())
	if err != nil {
		apperrors.Handle(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"items": output})
}

// Activate enables API-key access for an external client.
func (h *ClientHandler) Activate(c *gin.Context) {
	h.updateStatus(c, func(claims *auth.Claims) (clients.ListOutput, error) {
		return h.service.ActivateClient(c.Request.Context(), claims.UserID, c.Param("id"))
	})
}

// Deactivate revokes API-key access for an external client.
func (h *ClientHandler) Deactivate(c *gin.Context) {
	h.updateStatus(c, func(claims *auth.Claims) (clients.ListOutput, error) {
		return h.service.DeactivateClient(c.Request.Context(), claims.UserID, c.Param("id"))
	})
}

// RotateAPIKey replaces a client's API key and returns the new key once.
func (h *ClientHandler) RotateAPIKey(c *gin.Context) {
	claims, exists := auth.GetClaims(c)
	if !exists {
		apperrors.RespondWithError(c, apperrors.Unauthorized("User not authenticated"))
		return
	}
	if h.service == nil {
		apperrors.RespondWithError(c, apperrors.ConfigurationError("client service is not configured"))
		return
	}

	output, err := h.service.RotateClientAPIKey(c.Request.Context(), claims.UserID, c.Param("id"))
	if err != nil {
		apperrors.Handle(c, err)
		return
	}

	c.JSON(http.StatusOK, output)
}

func (h *ClientHandler) updateStatus(
	c *gin.Context,
	action func(*auth.Claims) (clients.ListOutput, error),
) {
	claims, exists := auth.GetClaims(c)
	if !exists {
		apperrors.RespondWithError(c, apperrors.Unauthorized("User not authenticated"))
		return
	}
	if h.service == nil {
		apperrors.RespondWithError(c, apperrors.ConfigurationError("client service is not configured"))
		return
	}

	output, err := action(claims)
	if err != nil {
		apperrors.Handle(c, err)
		return
	}

	c.JSON(http.StatusOK, output)
}
