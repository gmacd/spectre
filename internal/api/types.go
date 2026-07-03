package api

// SendRequest is the shape of a POST /v1/messages body.
type SendRequest struct {
	ConversationID string `json:"conversation_id"`
	Message        string `json:"message"`
}

// SendResponse is the shape of a successful POST /v1/messages response.
type SendResponse struct {
	ConversationID string `json:"conversation_id"`
	Reply          string `json:"reply"`
}

// ErrorResponse is the shape of an error response.
type ErrorResponse struct {
	Error string `json:"error"`
}
