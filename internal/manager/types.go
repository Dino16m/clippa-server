package manager

import "github.com/google/uuid"

type PartyCreateRequest struct {
	Name   string `json:"name"`
	Secret string `json:"secret"`
}

type GetPartyRequest struct {
	ID     string `json:"id"`
	Secret string `json:"secret"`
}

type AuthRequest struct {
	ID     string `json:"id"`
	Secret string `json:"secret"`
}

type AuthResponse struct {
	Token string `json:"token"`
}

type PartyResponse struct {
	ID            uuid.UUID `json:"id"`
	Name          string    `json:"name"`
	LeaderAddress string    `json:"leaderAddress,omitempty"`
	CertPEM       string    `json:"certPem,omitempty"`
	KeyPEM        string    `json:"keyPem,omitempty"`
}
