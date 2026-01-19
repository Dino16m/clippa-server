package manager

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/dino16m/clippa-server/internal/data"
	"github.com/dino16m/clippa-server/internal/service"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
)

type ManagerCtrl struct {
	store         *data.PartyStore
	logger        *logrus.Logger
	authStore     *AuthService
	partyProvider *service.PartyServiceProvider
}

func NewManagerCtrl(store *data.PartyStore, logger *logrus.Logger) *ManagerCtrl {
	return &ManagerCtrl{
		store:         store,
		logger:        logger,
		authStore:     NewAuthService(),
		partyProvider: service.NewPartyServiceProvider(store, logger),
	}
}

func (mc *ManagerCtrl) CreateParty(w http.ResponseWriter, r *http.Request) {

	var req PartyCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Secret = strings.TrimSpace(req.Secret)
	if req.Name == "" || req.Secret == "" {
		http.Error(w, "name and secret are required", http.StatusBadRequest)
		return
	}

	id := uuid.New()
	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Secret), bcrypt.DefaultCost)
	if err != nil {
		mc.logger.WithError(err).Error("failed to hash secret")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	party := data.Party{
		ID:            id,
		Name:          req.Name,
		Password:      string(hashed),
		LeaderAddress: "",
	}

	// generate a CA bundle for this party and store the CA cert/key
	ca, err := generateCaBundle(req.Name)
	if err != nil {
		mc.logger.WithError(err).Error("failed to generate CA bundle")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	party.CertPEM = base64.StdEncoding.EncodeToString(ca.CertPEM)
	party.KeyPEM = base64.StdEncoding.EncodeToString(ca.KeyPEM)

	if err := mc.store.Create(&party); err != nil {
		mc.logger.WithError(err).Error("failed to create party")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	resp := PartyResponse{
		ID:            party.ID,
		Name:          party.Name,
		LeaderAddress: party.LeaderAddress,
		CertPEM:       party.CertPEM,
		KeyPEM:        party.KeyPEM,
	}
	WriteJson(w, http.StatusCreated, resp)
}

func (mc *ManagerCtrl) GetParty(w http.ResponseWriter, r *http.Request) {
	req, err := getPartyRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	party, err := mc.store.Get(req.ID)
	if err != nil {
		mc.logger.WithError(err).WithField("id", req.ID).Warn("party not found")
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(party.Password), []byte(req.Secret)); err != nil {
		mc.logger.WithError(err).WithField("id", req.ID).Warn("invalid secret")
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	resp := PartyResponse{
		ID:            party.ID,
		Name:          party.Name,
		LeaderAddress: party.LeaderAddress,
		CertPEM:       party.CertPEM,
		KeyPEM:        party.KeyPEM,
	}
	WriteJson(w, http.StatusOK, resp)
}

func getPartyRequest(r *http.Request) (GetPartyRequest, error) {
	id := r.URL.Query().Get("id")
	secret := r.Header.Get("X-Secret")
	id = strings.TrimSpace(id)
	secret = strings.TrimSpace(secret)
	if id == "" || secret == "" {
		return GetPartyRequest{}, errors.New("id and secret are required")
	}
	return GetPartyRequest{
		ID:     id,
		Secret: secret,
	}, nil
}

func (mc *ManagerCtrl) Authenticate(w http.ResponseWriter, r *http.Request) {
	mc.logger.Info("authenticating")

	req, err := getPartyRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	party, err := mc.store.Get(req.ID)
	if err != nil {
		mc.logger.WithError(err).WithField("id", req.ID).Warn("party not found")
		http.Error(w, "invalid party", http.StatusUnauthorized)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(party.Password), []byte(req.Secret)); err != nil {
		mc.logger.WithError(err).WithField("id", req.ID).Warn("invalid secret")
		http.Error(w, "invalid party", http.StatusUnauthorized)
		return
	}

	token, _ := SecureRandomString(64)

	mc.authStore.SaveToken(req.ID, token)
	resp := AuthResponse{
		Token: token,
	}
	WriteJson(w, http.StatusOK, resp)
}

func (mc *ManagerCtrl) validatePartyMembership(w http.ResponseWriter, r *http.Request) (string, error) {
	// Validate token and party id from the websocket URL before upgrading
	q := r.URL.Query()
	token := strings.TrimSpace(q.Get("token"))
	idFromURL := strings.TrimSpace(q.Get("id"))
	if token == "" || idFromURL == "" {
		http.Error(w, "token and id are required", http.StatusBadRequest)
		return "", errors.New("token and id are required")
	}

	storedPartyID := mc.authStore.GetPartyId(token)
	if storedPartyID == "" {
		mc.logger.WithField("token", token).Warn("invalid or expired token")
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return "", errors.New("invalid or expired token")
	}

	if storedPartyID != idFromURL {
		mc.logger.WithField("token", token).WithField("expected", idFromURL).WithField("got", storedPartyID).Warn("token does not match party id")
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return "", errors.New("token does not match party id")
	}

	mc.authStore.DeleteToken(token)

	return storedPartyID, nil
}

func (mc *ManagerCtrl) JoinParty(w http.ResponseWriter, r *http.Request) {

	memberId := strings.TrimSpace(r.URL.Query().Get("memberId"))
	if memberId == "" {
		memberId = uuid.New().String()
	}
	storedPartyID, err := mc.validatePartyMembership(w, r)
	if err != nil {
		return
	}

	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		mc.logger.WithError(err).Error("websocket accept failed")
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	partyHandle := mc.partyProvider.JoinParty(storedPartyID, memberId)
	mc.logger.WithField("id", storedPartyID).Info("joined party with handle")
	defer partyHandle.Leave()
	ctx := r.Context()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	outbox := make(chan []byte)

	go func(conn *websocket.Conn, outbox chan<- []byte, ctx context.Context, cancel context.CancelFunc) {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				_, msg, err := conn.Read(ctx)
				if err != nil {
					cancel()
					return
				}
				outbox <- msg
			}
		}
	}(conn, outbox, ctx, cancel)

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-partyHandle.Inbox():
			if !ok {
				return
			}
			ctxWithTimeout, cancel := context.WithTimeout(ctx, 1*time.Second)
			defer cancel()
			if err := conn.Write(ctxWithTimeout, websocket.MessageText, msg); err != nil {
				return
			}
		case msg, ok := <-outbox:
			if !ok {
				return
			}
			err = partyHandle.HandleMessage(msg)
			if err != nil {
				ctxWithTimeout, cancel := context.WithTimeout(ctx, 1*time.Second)
				defer cancel()
				conn.Write(ctxWithTimeout, websocket.MessageText, service.ErrorMessage(err.Error()))
			}
		}
	}
}

func (mc *ManagerCtrl) RegisterRoutes(globalMux *http.ServeMux) {
	localMux := http.NewServeMux()

	localMux.HandleFunc("POST /", mc.CreateParty)
	localMux.HandleFunc("GET /", mc.GetParty)
	localMux.HandleFunc("GET /join", mc.JoinParty)
	localMux.HandleFunc("GET /auth", mc.Authenticate)
	globalMux.Handle("/parties/", http.StripPrefix("/parties", localMux))
}
