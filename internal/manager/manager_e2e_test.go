package manager_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/glebarez/sqlite"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/dino16m/clippa-server/internal/data"
	"github.com/dino16m/clippa-server/internal/manager"
)

func createParty(t *testing.T, base, name, secret string) string {
	t.Helper()
	createReq := map[string]string{"name": name, "secret": secret}
	b, _ := json.Marshal(createReq)
	resp, err := http.Post(base+"/api/parties/", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("create party request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", resp.StatusCode)
	}
	var pr manager.PartyResponse
	if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
		t.Fatalf("decode create resp: %v", err)
	}
	return pr.ID.String()
}

func authenticate(t *testing.T, base, idStr, secret string) string {
	t.Helper()
	req, err := http.NewRequest("GET", base+"/api/parties/auth?id="+idStr, nil)
	if err != nil {
		t.Fatalf("auth request creation: %v", err)
	}
	req.Header.Set("X-Secret", secret)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("auth request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected auth 200, got %d", resp.StatusCode)
	}
	var ar manager.AuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&ar); err != nil {
		t.Fatalf("decode auth resp: %v", err)
	}
	return ar.Token
}

func joinParty(t *testing.T, wsBase, idStr, token string) (*websocket.Conn, context.Context, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	u := wsBase + "/api/parties/join?id=" + url.QueryEscape(idStr) + "&token=" + url.QueryEscape(token)
	wsConn, _, err := websocket.Dial(ctx, u, nil)
	if err != nil {
		cancel()
		t.Fatalf("websocket dial: %v", err)
	}
	return wsConn, ctx, cancel
}

func setupServer(t *testing.T, mc *manager.ManagerCtrl) (base, wsBase string, shutdown func()) {
	t.Helper()

	globalMux := http.NewServeMux()
	mc.RegisterRoutes(globalMux)

	topMux := http.NewServeMux()
	topMux.Handle("/api/", http.StripPrefix("/api", globalMux))

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	srv := &http.Server{Handler: topMux}
	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			t.Logf("server serve: %v", err)
		}
	}()

	base = fmt.Sprintf("http://%s", ln.Addr().String())
	wsBase = fmt.Sprintf("ws://%s", ln.Addr().String())

	shutdown = func() {
		srv.Shutdown(context.Background())
	}
	return
}

func TestJoinPartyWebsocket(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&data.Party{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	store := data.NewPartyStore(db)
	logger := logrus.New()
	mc := manager.NewManagerCtrl(store, logger)

	base, wsBase, shutdown := setupServer(t, mc)
	defer shutdown()

	id := createParty(t, base, "test-party", "s3cr3t")
	token := authenticate(t, base, id, "s3cr3t")

	wsConn, ctx, cancel := joinParty(t, wsBase, id, token)
	defer cancel()
	defer wsConn.Close(websocket.StatusNormalClosure, "")

	// Simulate a second member joining
	token2 := authenticate(t, base, id, "s3cr3t")
	wsConn2, ctx2, cancel2 := joinParty(t, wsBase, id, token2)
	defer cancel2()
	defer wsConn2.Close(websocket.StatusNormalClosure, "")

	// Send ping from the first member
	pingMsg := `{"messageType":"ping"}`
	if err := wsConn.Write(ctx, websocket.MessageText, []byte(pingMsg)); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Read from the second member's connection to see if it receives the pong
	mt, msg, err := wsConn2.Read(ctx2)
	if err != nil {
		t.Fatalf("read from second connection: %v", err)
	}
	if mt != websocket.MessageText {
		t.Fatalf("unexpected message type: mt=%d", mt)
	}
	var receivedMsg map[string]interface{}
	if err := json.Unmarshal(msg, &receivedMsg); err != nil {
		t.Fatalf("failed to unmarshal echoed message: %v", err)
	}
	if receivedMsg["messageType"] != "ping" {
		t.Fatalf("expected ping back, got %s", receivedMsg["messageType"])
	}
}

func TestGetPartyRequiresCorrectSecret(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&data.Party{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	store := data.NewPartyStore(db)
	logger := logrus.New()
	mc := manager.NewManagerCtrl(store, logger)

	base, _, shutdown := setupServer(t, mc)
	defer shutdown()

	id := createParty(t, base, "secret-test", "correct-secret")

	// Request with wrong secret -> expect 401
	wrongReq, err := http.NewRequest("GET", base+"/api/parties/?id="+id, nil)
	if err != nil {
		t.Fatalf("wrong secret request creation: %v", err)
	}
	wrongReq.Header.Set("X-Secret", "bad-secret")
	resp, err := http.DefaultClient.Do(wrongReq)
	if err != nil {
		t.Fatalf("wrong secret request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for wrong secret, got %d", resp.StatusCode)
	}

	// Request with correct secret -> expect 200 and valid ID in response
	correctReq, err := http.NewRequest("GET", base+"/api/parties/?id="+id, nil)
	if err != nil {
		t.Fatalf("correct secret request creation: %v", err)
	}
	correctReq.Header.Set("X-Secret", "correct-secret")
	resp2, err := http.DefaultClient.Do(correctReq)
	if err != nil {
		t.Fatalf("correct secret request: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for correct secret, got %d", resp2.StatusCode)
	}
	var pr manager.PartyResponse
	if err := json.NewDecoder(resp2.Body).Decode(&pr); err != nil {
		t.Fatalf("decode get resp: %v", err)
	}
	if pr.ID.String() != id {
		t.Fatalf("expected id %s, got %s", id, pr.ID.String())
	}
}
