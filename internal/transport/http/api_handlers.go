package http

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"github.com/vovakirdan/wirechat-server/internal/auth"
)

// APIHandlers provides HTTP handlers for REST API endpoints.
type APIHandlers struct {
	authService *auth.Service
	log         *zerolog.Logger
}

// NewAPIHandlers creates a new API handlers instance.
func NewAPIHandlers(authService *auth.Service, logger *zerolog.Logger) *APIHandlers {
	return &APIHandlers{
		authService: authService,
		log:         logger,
	}
}

// RegisterRequest represents the registration request body.
type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=32"`
	Password string `json:"password" binding:"required,min=6"`
}

// LoginRequest represents the login request body.
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// AuthResponse represents the authentication response body.
type AuthResponse struct {
	Token string `json:"token"`
}

// ErrorResponse represents an error response body.
type ErrorResponse struct {
	Error string `json:"error"`
}

// Register handles user registration.
// POST /api/register
func (h *APIHandlers) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.log.Debug().Err(err).Msg("invalid register request")
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request body"})
		return
	}

	token, err := h.authService.Register(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		if errors.Is(err, auth.ErrUserExists) {
			c.JSON(http.StatusConflict, ErrorResponse{Error: "user already exists"})
			return
		}
		h.log.Error().Err(err).Str("username", req.Username).Msg("failed to register user")
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
		return
	}

	h.log.Info().Str("username", req.Username).Msg("user registered successfully")
	c.JSON(http.StatusCreated, AuthResponse{Token: token})
}

// Login handles user login.
// POST /api/login
func (h *APIHandlers) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.log.Debug().Err(err).Msg("invalid login request")
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request body"})
		return
	}

	token, err := h.authService.Login(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "invalid credentials"})
			return
		}
		h.log.Error().Err(err).Str("username", req.Username).Msg("failed to login user")
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
		return
	}

	h.log.Info().Str("username", req.Username).Msg("user logged in successfully")
	c.JSON(http.StatusOK, AuthResponse{Token: token})
}

// GuestLogin creates a guest user and returns a token.
// POST /api/guest
func (h *APIHandlers) GuestLogin(c *gin.Context) {
	token, sessionID, err := h.authService.CreateGuestUser(c.Request.Context())
	if err != nil {
		h.log.Error().Err(err).Msg("failed to create guest user")
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
		return
	}

	// Set session cookie
	c.SetCookie(
		"guest_session",
		sessionID,
		3600*24*7, // 7 days
		"/",
		"",
		false, // secure (set to true in production with HTTPS)
		true,  // httpOnly
	)

	h.log.Info().Str("session_id", sessionID).Msg("guest user created")
	c.JSON(http.StatusOK, AuthResponse{Token: token})
}
