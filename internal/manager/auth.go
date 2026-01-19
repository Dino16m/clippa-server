package manager

import (
	"sync"
)

type AuthService struct {
	mutex      *sync.RWMutex
	identities map[string]string
}

func NewAuthService() *AuthService {
	return &AuthService{
		mutex:      &sync.RWMutex{},
		identities: map[string]string{},
	}
}

func (a *AuthService) SaveToken(partyId, token string) {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	a.identities[token] = partyId
}

func (a *AuthService) GetPartyId(token string) string {
	a.mutex.RLock()
	defer a.mutex.RUnlock()
	return a.identities[token]
}

func (a *AuthService) DeleteToken(token string) {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	delete(a.identities, token)
}
