package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"go-web/model"
	"go-web/resp"
	"go-web/router"
	"go-web/testutil"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestRouter() {
	gin.SetMode(gin.TestMode)
	resp.Init()
	model.DB = testutil.SetupTestDB()
	testutil.SetupTestAuth()
}

func issueAuth(t *testing.T, name string, role int, vip ...int) string {
	t.Helper()
	vipLevel := 0
	if len(vip) > 0 {
		vipLevel = vip[0]
	}
	user := &model.User{Name: name, Role: role, VipLevel: vipLevel, Status: model.StatusEnabled}
	require.NoError(t, user.SetPassword("secret"))
	require.NoError(t, user.Insert())
	pair, err := model.IssueTokenPair(user)
	require.NoError(t, err)
	return "Bearer " + pair.AccessToken
}

func TestCreateUser(t *testing.T) {
	setupTestRouter()
	defer testutil.CleanupTestDB(model.DB)

	r := router.SetRouter()
	auth := issueAuth(t, "admin-user", model.RoleAdmin)

	body, _ := json.Marshal(model.User{Name: "Alice"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/users", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", auth)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestCreateUserForbidden(t *testing.T) {
	setupTestRouter()
	defer testutil.CleanupTestDB(model.DB)

	r := router.SetRouter()
	auth := issueAuth(t, "bob", model.RoleUser)

	body, _ := json.Marshal(model.User{Name: "Alice"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/users", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", auth)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestLogin(t *testing.T) {
	setupTestRouter()
	defer testutil.CleanupTestDB(model.DB)

	user := &model.User{Name: "alice", Role: model.RoleUser, Status: model.StatusEnabled}
	require.NoError(t, user.SetPassword("secret"))
	require.NoError(t, user.Insert())

	r := router.SetRouter()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/login", bytes.NewBufferString(`{"name":"alice","password":"secret"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "access_token")
	assert.Contains(t, w.Body.String(), "capabilities")
}

func TestMe(t *testing.T) {
	setupTestRouter()
	defer testutil.CleanupTestDB(model.DB)

	r := router.SetRouter()
	auth := issueAuth(t, "alice", model.RoleUser, model.Vip1)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/me", nil)
	req.Header.Set("Authorization", auth)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"can_use_ai":true`)
	assert.Contains(t, w.Body.String(), `"can_edit":true`)
}

func TestCanvasCRUD(t *testing.T) {
	setupTestRouter()
	defer testutil.CleanupTestDB(model.DB)

	r := router.SetRouter()
	auth := issueAuth(t, "painter", model.RoleUser)

	create := func(title string) int64 {
		t.Helper()
		doc := json.RawMessage(`{"activePageId":"page-1","pageIds":["page-1"],"pages":{}}`)
		createBody, _ := json.Marshal(map[string]any{
			"title":    title,
			"document": doc,
		})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/canvases", bytes.NewBuffer(createBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", auth)
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusCreated, w.Code)

		var created struct {
			Data model.Canvas `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
		require.NotZero(t, created.Data.ID)
		return created.Data.ID
	}

	id1 := create("demo-1")
	id2 := create("demo-2")
	require.NotEqual(t, id1, id2)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/canvases", nil)
	req.Header.Set("Authorization", auth)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var listed struct {
		Data       []model.CanvasSummary `json:"data"`
		Pagination *resp.Pagination      `json:"pagination"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &listed))
	assert.Len(t, listed.Data, 2)
	assert.Equal(t, 2, listed.Pagination.Total)

	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", fmt.Sprintf("/api/v1/canvases/%d", id1), nil)
	req.Header.Set("Authorization", auth)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "demo-1")
}

func TestMaterialWriteRequiresVIP2(t *testing.T) {
	setupTestRouter()
	defer testutil.CleanupTestDB(model.DB)

	r := router.SetRouter()
	auth := issueAuth(t, "vip1-user", model.RoleUser, model.Vip1)

	body := []byte(`{"key":"x","title":"X","kind":"image"}`)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/materials", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", auth)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)

	auth2 := issueAuth(t, "vip2-user", model.RoleUser, model.Vip2)
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/v1/materials", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", auth2)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestRefresh(t *testing.T) {
	setupTestRouter()
	defer testutil.CleanupTestDB(model.DB)

	user := &model.User{Name: "alice", Role: model.RoleUser, Status: model.StatusEnabled}
	require.NoError(t, user.SetPassword("secret"))
	require.NoError(t, user.Insert())
	pair, err := model.IssueTokenPair(user)
	require.NoError(t, err)

	r := router.SetRouter()
	body, _ := json.Marshal(map[string]string{"refresh_token": pair.RefreshToken})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/refresh", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRefreshRateLimitByIP(t *testing.T) {
	setupTestRouter()
	defer testutil.CleanupTestDB(model.DB)

	r := router.SetRouter()
	for i := 1; i <= 11; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/refresh", bytes.NewBufferString(`{}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		if i <= 10 {
			assert.Equal(t, http.StatusBadRequest, w.Code)
			continue
		}
		assert.Equal(t, http.StatusTooManyRequests, w.Code)
		assert.NotEmpty(t, w.Header().Get("Retry-After"))
		assert.Contains(t, w.Body.String(), resp.CodeRateLimited)
	}
}

func TestLoginRateLimitByIP(t *testing.T) {
	setupTestRouter()
	defer testutil.CleanupTestDB(model.DB)

	r := router.SetRouter()
	for i := 1; i <= 11; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/login", bytes.NewBufferString(`{}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		if i <= 10 {
			assert.Equal(t, http.StatusBadRequest, w.Code)
			continue
		}
		assert.Equal(t, http.StatusTooManyRequests, w.Code)
	}
}

func TestRegisterRateLimitByIP(t *testing.T) {
	setupTestRouter()
	defer testutil.CleanupTestDB(model.DB)

	r := router.SetRouter()
	for i := 1; i <= 6; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/register", bytes.NewBufferString(`{}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		if i <= 5 {
			assert.Equal(t, http.StatusBadRequest, w.Code)
			continue
		}
		assert.Equal(t, http.StatusTooManyRequests, w.Code)
	}
}
