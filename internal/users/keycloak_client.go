package users

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// KeycloakClient faz chamadas à Keycloak Admin REST API.
type KeycloakClient struct {
	baseURL      string
	realm        string
	clientID     string
	clientSecret string

	mu          sync.Mutex
	adminToken  string
	tokenExpiry time.Time
}

func NewKeycloakClient(baseURL, realm, clientID, clientSecret string) *KeycloakClient {
	return &KeycloakClient{
		baseURL:      baseURL,
		realm:        realm,
		clientID:     clientID,
		clientSecret: clientSecret,
	}
}

// ── Admin token (client credentials flow) ──────────────────────────────────

func (kc *KeycloakClient) getAdminToken() (string, error) {
	kc.mu.Lock()
	defer kc.mu.Unlock()

	if kc.adminToken != "" && time.Now().Before(kc.tokenExpiry) {
		return kc.adminToken, nil
	}

	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", kc.clientID)
	form.Set("client_secret", kc.clientSecret)

	resp, err := http.Post( //nolint:gosec
		fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token", kc.baseURL, kc.realm),
		"application/x-www-form-urlencoded",
		strings.NewReader(form.Encode()),
	)
	if err != nil {
		return "", fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("token decode failed: %w", err)
	}
	if result.AccessToken == "" {
		return "", fmt.Errorf("empty token from Keycloak — check client_secret and service account")
	}

	kc.adminToken = result.AccessToken
	kc.tokenExpiry = time.Now().Add(time.Duration(result.ExpiresIn-10) * time.Second)

	return kc.adminToken, nil
}

// ── Helpers ─────────────────────────────────────────────────────────────────

func (kc *KeycloakClient) adminURL(path string) string {
	return fmt.Sprintf("%s/admin/realms/%s%s", kc.baseURL, kc.realm, path)
}

func (kc *KeycloakClient) do(method, path string, body any) (*http.Response, error) {
	token, err := kc.getAdminToken()
	if err != nil {
		return nil, err
	}

	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, kc.adminURL(path), bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	return http.DefaultClient.Do(req)
}

// ── User operations ──────────────────────────────────────────────────────────

func (kc *KeycloakClient) CreateUser(input RegisterInput) (string, error) {
	payload := map[string]any{
		"username":      input.Username,
		"email":         input.Email,
		"firstName":     input.FirstName,
		"lastName":      input.LastName,
		"enabled":       true,
		"emailVerified": true,
		"credentials": []map[string]any{
			{"type": "password", "value": input.Password, "temporary": false},
		},
	}

	resp, err := kc.do("POST", "/users", payload)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusConflict {
		return "", fmt.Errorf("username or email already exists")
	}
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("create user failed (%d): %s", resp.StatusCode, string(body))
	}

	// Keycloak retorna o ID no header Location: .../users/{id}
	location := resp.Header.Get("Location")
	parts := strings.Split(location, "/")
	return parts[len(parts)-1], nil
}

func (kc *KeycloakClient) GetUsers() ([]User, error) {
	resp, err := kc.do("GET", "/users?max=100", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var users []User
	if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
		return nil, err
	}

	// enriquecer cada utilizador com as suas roles
	for i := range users {
		roles, err := kc.GetUserRoles(users[i].ID)
		if err == nil {
			users[i].Roles = roles
		}
	}

	return users, nil
}

func (kc *KeycloakClient) GetUser(id string) (*User, error) {
	resp, err := kc.do("GET", "/users/"+id, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	var user User
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, err
	}

	// busca as roles do utilizador
	roles, err := kc.GetUserRoles(id)
	if err == nil {
		user.Roles = roles
	}

	return &user, nil
}

func (kc *KeycloakClient) DeleteUser(id string) error {
	resp, err := kc.do("DELETE", "/users/"+id, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("delete failed with status %d", resp.StatusCode)
	}
	return nil
}

// ── Role operations ──────────────────────────────────────────────────────────

func (kc *KeycloakClient) GetUserRoles(userID string) ([]string, error) {
	resp, err := kc.do("GET", "/users/"+userID+"/role-mappings/realm", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var roles []struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&roles); err != nil {
		return nil, err
	}

	names := make([]string, 0, len(roles))
	for _, r := range roles {
		names = append(names, r.Name)
	}
	return names, nil
}

func (kc *KeycloakClient) AssignRoles(userID string, roleNames []string) error {
	// busca os objetos de role pelo nome
	roles, err := kc.resolveRoles(roleNames)
	if err != nil {
		return err
	}

	resp, err := kc.do("POST", "/users/"+userID+"/role-mappings/realm", roles)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("assign roles failed (%d): %s", resp.StatusCode, string(body))
	}
	return nil
}

func (kc *KeycloakClient) RemoveRoles(userID string, roleNames []string) error {
	roles, err := kc.resolveRoles(roleNames)
	if err != nil {
		return err
	}

	resp, err := kc.do("DELETE", "/users/"+userID+"/role-mappings/realm", roles)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (kc *KeycloakClient) resolveRoles(names []string) ([]map[string]any, error) {
	var result []map[string]any
	for _, name := range names {
		resp, err := kc.do("GET", "/roles/"+name, nil)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusNotFound {
			return nil, fmt.Errorf("role %q not found in realm", name)
		}

		var role map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&role); err != nil {
			return nil, err
		}
		result = append(result, role)
	}
	return result, nil
}
