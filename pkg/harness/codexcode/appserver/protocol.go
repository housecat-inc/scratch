package appserver

import (
	"encoding/json"
	"fmt"
)

const (
	LoginTypeAPIKey     = "apiKey"
	LoginTypeChatGPT    = "chatgpt"
	LoginTypeDeviceCode = "chatgptDeviceCode"

	MethodAccountLoginCancel    = "account/login/cancel"
	MethodAccountLoginCompleted = "account/login/completed"
	MethodAccountLoginStart     = "account/login/start"
	MethodInitialize            = "initialize"
	MethodInitialized           = "initialized"
)

type ClientInfo struct {
	Name    string `json:"name"`
	Title   string `json:"title"`
	Version string `json:"version"`
}

type InitializeParams struct {
	ClientInfo ClientInfo `json:"clientInfo"`
}

type InitializeResponse struct {
	CodexHome string `json:"codexHome"`
	UserAgent string `json:"userAgent"`
}

type LoginCancelParams struct {
	LoginID string `json:"loginId"`
}

type LoginCompleted struct {
	Error   *string `json:"error"`
	LoginID *string `json:"loginId"`
	Success bool    `json:"success"`
}

type LoginParams struct {
	APIKey string `json:"apiKey,omitempty"`
	Type   string `json:"type"`
}

type LoginResponse struct {
	AuthURL         string `json:"authUrl,omitempty"`
	LoginID         string `json:"loginId,omitempty"`
	Type            string `json:"type"`
	UserCode        string `json:"userCode,omitempty"`
	VerificationURL string `json:"verificationUrl,omitempty"`
}

type Notification struct {
	Method string
	Params json.RawMessage
}

type RPCError struct {
	Code    int             `json:"code"`
	Data    json.RawMessage `json:"data,omitempty"`
	Message string          `json:"message"`
}

func (e *RPCError) Error() string {
	return fmt.Sprintf("app-server error %d: %s", e.Code, e.Message)
}

type message struct {
	Error  *RPCError       `json:"error,omitempty"`
	ID     json.RawMessage `json:"id,omitempty"`
	Method string          `json:"method,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
}
