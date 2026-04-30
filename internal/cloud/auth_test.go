package cloud

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func newTestServer(t *testing.T) *CloudServer {
	t.Helper()
	store := newTestStore(t)
	key, err := generateSigningKey()
	if err != nil {
		t.Fatalf("generateSigningKey: %v", err)
	}
	return &CloudServer{
		router:      gin.New(),
		videoStore:  store,
		jwtKey:      key,
		tokenExpiry: 5 * time.Minute,
	}
}

func TestGenerateSigningKey(t *testing.T) {
	key, err := generateSigningKey()
	if err != nil {
		t.Fatalf("generateSigningKey: %v", err)
	}
	if len(key) != 32 {
		t.Errorf("got key length %d, want 32", len(key))
	}
}

func TestCreateAndValidateToken(t *testing.T) {
	s := newTestServer(t)
	user := &User{ID: "uid1", Username: "alice"}

	token, err := s.createToken(user)
	if err != nil {
		t.Fatalf("createToken: %v", err)
	}

	claims, err := s.validateToken(token)
	if err != nil {
		t.Fatalf("validateToken: %v", err)
	}
	if claims.UserID != "uid1" || claims.Username != "alice" {
		t.Errorf("unexpected claims: %+v", claims)
	}
}

func TestValidateTokenExpired(t *testing.T) {
	s := newTestServer(t)
	s.tokenExpiry = -1 * time.Second
	user := &User{ID: "uid1", Username: "alice"}

	token, err := s.createToken(user)
	if err != nil {
		t.Fatalf("createToken: %v", err)
	}

	_, err = s.validateToken(token)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestValidateTokenInvalidSignature(t *testing.T) {
	s := newTestServer(t)
	user := &User{ID: "uid1", Username: "alice"}

	token, err := s.createToken(user)
	if err != nil {
		t.Fatalf("createToken: %v", err)
	}

	s2 := newTestServer(t)
	_, err = s2.validateToken(token)
	if err == nil {
		t.Fatal("expected error for wrong signing key")
	}
}

func TestAuthMiddlewareNoToken(t *testing.T) {
	s := newTestServer(t)

	r := gin.New()
	r.GET("/test", s.authMiddleware(), func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("got status %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestAuthMiddlewareValidToken(t *testing.T) {
	s := newTestServer(t)
	user := &User{ID: "uid1", Username: "alice"}
	token, _ := s.createToken(user)

	r := gin.New()
	r.GET("/test", s.authMiddleware(), func(c *gin.Context) {
		c.JSON(200, gin.H{
			"user_id":  c.GetString("user_id"),
			"username": c.GetString("username"),
		})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.AddCookie(&http.Cookie{Name: "auth_token", Value: token})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusOK)
	}

	var body map[string]string
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["user_id"] != "uid1" || body["username"] != "alice" {
		t.Errorf("unexpected body: %v", body)
	}
}

func TestAuthMiddlewareExpiredToken(t *testing.T) {
	s := newTestServer(t)
	s.tokenExpiry = -1 * time.Second
	user := &User{ID: "uid1", Username: "alice"}
	token, _ := s.createToken(user)

	s.tokenExpiry = 5 * time.Minute

	r := gin.New()
	r.GET("/test", s.authMiddleware(), func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.AddCookie(&http.Cookie{Name: "auth_token", Value: token})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("got status %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestAuthMiddlewareRefreshesNearExpiry(t *testing.T) {
	s := newTestServer(t)
	s.tokenExpiry = 30 * time.Second
	user := &User{ID: "uid1", Username: "alice"}
	token, _ := s.createToken(user)

	s.tokenExpiry = 5 * time.Minute

	r := gin.New()
	r.GET("/test", s.authMiddleware(), func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.AddCookie(&http.Cookie{Name: "auth_token", Value: token})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusOK)
	}

	cookies := w.Result().Cookies()
	found := false
	for _, c := range cookies {
		if c.Name == "auth_token" && c.Value != token {
			found = true
		}
	}
	if !found {
		t.Error("expected refreshed auth_token cookie")
	}
}

func TestHandleLoginSuccess(t *testing.T) {
	s := newTestServer(t)
	s.videoStore.CreateUser("alice", "secret123", "admin")

	r := gin.New()
	r.POST("/auth/login", s.handleLogin)

	body := `{"username":"alice","password":"secret123"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusOK)
	}

	cookies := w.Result().Cookies()
	found := false
	for _, c := range cookies {
		if c.Name == "auth_token" && c.Value != "" {
			found = true
		}
	}
	if !found {
		t.Error("expected auth_token cookie in response")
	}
}

func TestHandleLoginWrongPassword(t *testing.T) {
	s := newTestServer(t)
	s.videoStore.CreateUser("alice", "secret123", "admin")

	r := gin.New()
	r.POST("/auth/login", s.handleLogin)

	body := `{"username":"alice","password":"wrong"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("got status %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestHandleLoginNonexistentUser(t *testing.T) {
	s := newTestServer(t)

	r := gin.New()
	r.POST("/auth/login", s.handleLogin)

	body := `{"username":"nobody","password":"pass"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("got status %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestAdminOnlyAllowsAdmin(t *testing.T) {
	s := newTestServer(t)
	user := &User{ID: "uid1", Username: "alice", Role: "admin"}
	token, _ := s.createToken(user)

	r := gin.New()
	r.GET("/test", s.authMiddleware(), s.adminOnly(), func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.AddCookie(&http.Cookie{Name: "auth_token", Value: token})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("got status %d, want %d", w.Code, http.StatusOK)
	}
}

func TestAdminOnlyRejectsGuest(t *testing.T) {
	s := newTestServer(t)
	user := &User{ID: "uid2", Username: "bob", Role: "guest"}
	token, _ := s.createToken(user)

	r := gin.New()
	r.GET("/test", s.authMiddleware(), s.adminOnly(), func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.AddCookie(&http.Cookie{Name: "auth_token", Value: token})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("got status %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestHandleCreateUser(t *testing.T) {
	s := newTestServer(t)
	admin := &User{ID: "uid1", Username: "admin", Role: "admin"}
	token, _ := s.createToken(admin)

	r := gin.New()
	r.POST("/api/admin/users", s.authMiddleware(), s.adminOnly(), s.handleCreateUser)

	body := `{"username":"newuser","password":"pass123","role":"guest"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/admin/users", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "auth_token", Value: token})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("got status %d, want %d, body: %s", w.Code, http.StatusCreated, w.Body.String())
	}

	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["username"] != "newuser" || resp["role"] != "guest" {
		t.Errorf("unexpected response: %v", resp)
	}
}

func TestMemberOrAdminAllowsMember(t *testing.T) {
	s := newTestServer(t)
	user := &User{ID: "uid3", Username: "charlie", Role: "member"}
	token, _ := s.createToken(user)

	r := gin.New()
	r.POST("/test", s.authMiddleware(), s.memberOrAdmin(), func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/test", nil)
	req.AddCookie(&http.Cookie{Name: "auth_token", Value: token})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("got status %d, want %d", w.Code, http.StatusOK)
	}
}

func TestMemberOrAdminRejectsGuest(t *testing.T) {
	s := newTestServer(t)
	user := &User{ID: "uid4", Username: "dave", Role: "guest"}
	token, _ := s.createToken(user)

	r := gin.New()
	r.POST("/test", s.authMiddleware(), s.memberOrAdmin(), func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/test", nil)
	req.AddCookie(&http.Cookie{Name: "auth_token", Value: token})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("got status %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestHandleCreateUserGuestRejected(t *testing.T) {
	s := newTestServer(t)
	guest := &User{ID: "uid2", Username: "guest", Role: "guest"}
	token, _ := s.createToken(guest)

	r := gin.New()
	r.POST("/api/admin/users", s.authMiddleware(), s.adminOnly(), s.handleCreateUser)

	body := `{"username":"newuser","password":"pass123","role":"guest"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/admin/users", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "auth_token", Value: token})
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("got status %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestHandleLogout(t *testing.T) {
	s := newTestServer(t)

	r := gin.New()
	r.POST("/auth/logout", s.handleLogout)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/auth/logout", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusOK)
	}

	cookies := w.Result().Cookies()
	for _, c := range cookies {
		if c.Name == "auth_token" && c.MaxAge < 0 {
			return
		}
	}
	t.Error("expected auth_token cookie to be cleared")
}
