// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package hub

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	state "github.com/ptone/scion-agent/pkg/agent/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient_FromEnvironment(t *testing.T) {
	// Save and restore env vars
	origEndpoint := os.Getenv(EnvHubEndpoint)
	origURL := os.Getenv(EnvHubURL)
	origToken := os.Getenv(EnvHubToken)
	origAgentID := os.Getenv(EnvAgentID)
	defer func() {
		os.Setenv(EnvHubEndpoint, origEndpoint)
		os.Setenv(EnvHubURL, origURL)
		os.Setenv(EnvHubToken, origToken)
		os.Setenv(EnvAgentID, origAgentID)
	}()

	t.Run("missing env vars returns nil", func(t *testing.T) {
		os.Unsetenv(EnvHubEndpoint)
		os.Unsetenv(EnvHubURL)
		os.Unsetenv(EnvHubToken)
		os.Unsetenv(EnvAgentID)

		client := NewClient()
		assert.Nil(t, client)
	})

	t.Run("missing token returns nil", func(t *testing.T) {
		os.Unsetenv(EnvHubEndpoint)
		os.Setenv(EnvHubURL, "http://hub.example.com")
		os.Unsetenv(EnvHubToken)
		os.Unsetenv(EnvAgentID)

		client := NewClient()
		assert.Nil(t, client)
	})

	t.Run("missing agentID returns nil", func(t *testing.T) {
		os.Setenv(EnvHubEndpoint, "http://hub.example.com")
		os.Setenv(EnvHubToken, "test-token")
		os.Unsetenv(EnvAgentID)

		client := NewClient()
		assert.Nil(t, client, "should not create client without agent ID (local agent scenario)")
	})

	t.Run("with all env vars returns client", func(t *testing.T) {
		os.Unsetenv(EnvHubEndpoint)
		os.Setenv(EnvHubURL, "http://hub.example.com")
		os.Setenv(EnvHubToken, "test-token")
		os.Setenv(EnvAgentID, "agent-123")

		client := NewClient()
		require.NotNil(t, client)
		assert.True(t, client.IsConfigured())
	})

	t.Run("prefers SCION_HUB_ENDPOINT over SCION_HUB_URL", func(t *testing.T) {
		os.Setenv(EnvHubEndpoint, "http://endpoint.example.com")
		os.Setenv(EnvHubURL, "http://url.example.com")
		os.Setenv(EnvHubToken, "test-token")
		os.Setenv(EnvAgentID, "agent-123")

		client := NewClient()
		require.NotNil(t, client)
		assert.Equal(t, "http://endpoint.example.com", client.hubURL)
	})

	t.Run("falls back to SCION_HUB_URL when SCION_HUB_ENDPOINT not set", func(t *testing.T) {
		os.Unsetenv(EnvHubEndpoint)
		os.Setenv(EnvHubURL, "http://url.example.com")
		os.Setenv(EnvHubToken, "test-token")
		os.Setenv(EnvAgentID, "agent-123")

		client := NewClient()
		require.NotNil(t, client)
		assert.Equal(t, "http://url.example.com", client.hubURL)
	})
}

func TestNewClientWithConfig(t *testing.T) {
	client := NewClientWithConfig("http://hub.example.com", "test-token", "agent-123")

	require.NotNil(t, client)
	assert.True(t, client.IsConfigured())
}

func TestClient_IsConfigured(t *testing.T) {
	t.Run("nil client", func(t *testing.T) {
		var c *Client
		assert.False(t, c.IsConfigured())
	})

	t.Run("empty client", func(t *testing.T) {
		c := &Client{}
		assert.False(t, c.IsConfigured())
	})

	t.Run("missing agentID", func(t *testing.T) {
		c := NewClientWithConfig("http://hub.example.com", "token", "")
		assert.False(t, c.IsConfigured())
	})

	t.Run("missing token", func(t *testing.T) {
		c := NewClientWithConfig("http://hub.example.com", "", "agent-123")
		assert.False(t, c.IsConfigured())
	})

	t.Run("missing hubURL", func(t *testing.T) {
		c := NewClientWithConfig("", "token", "agent-123")
		assert.False(t, c.IsConfigured())
	})

	t.Run("all fields set", func(t *testing.T) {
		c := NewClientWithConfig("http://hub.example.com", "token", "agent-123")
		assert.True(t, c.IsConfigured())
	})
}

func TestClient_UpdateStatus(t *testing.T) {
	// Create a test server
	var receivedStatus StatusUpdate
	var receivedToken string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check request
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/v1/agents/agent-123/status", r.URL.Path)
		receivedToken = r.Header.Get("X-Scion-Agent-Token")

		// Parse body
		err := json.NewDecoder(r.Body).Decode(&receivedStatus)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClientWithConfig(server.URL, "test-token", "agent-123")

	err := client.UpdateStatus(context.Background(), StatusUpdate{
		Phase:    state.PhaseRunning,
		Activity: state.ActivityIdle,
		Status:   "idle",
		Message:  "test message",
	})

	require.NoError(t, err)
	assert.Equal(t, "test-token", receivedToken)
	assert.Equal(t, state.PhaseRunning, receivedStatus.Phase)
	assert.Equal(t, state.ActivityIdle, receivedStatus.Activity)
	assert.Equal(t, "idle", receivedStatus.Status)
	assert.Equal(t, "test message", receivedStatus.Message)
}

func TestClient_UpdateStatus_Errors(t *testing.T) {
	t.Run("not configured", func(t *testing.T) {
		client := &Client{}
		err := client.UpdateStatus(context.Background(), StatusUpdate{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not configured")
	})

	t.Run("no agent ID", func(t *testing.T) {
		client := NewClientWithConfig("http://hub.example.com", "test-token", "")
		err := client.UpdateStatus(context.Background(), StatusUpdate{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not configured")
	})

	t.Run("server error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("internal error"))
		}))
		defer server.Close()

		client := NewClientWithConfig(server.URL, "test-token", "agent-123")
		err := client.UpdateStatus(context.Background(), StatusUpdate{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "500")
	})
}

func TestClient_ReportState(t *testing.T) {
	var lastPayload map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&lastPayload)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClientWithConfig(server.URL, "test-token", "agent-123")
	ctx := context.Background()

	t.Run("running/idle", func(t *testing.T) {
		err := client.ReportState(ctx, state.PhaseRunning, state.ActivityIdle, "ready")
		require.NoError(t, err)
		assert.Equal(t, "running", lastPayload["phase"])
		assert.Equal(t, "idle", lastPayload["activity"])
		assert.Equal(t, "idle", lastPayload["status"])
		assert.Equal(t, "ready", lastPayload["message"])
	})

	t.Run("stopped", func(t *testing.T) {
		err := client.ReportState(ctx, state.PhaseStopped, "", "session ended")
		require.NoError(t, err)
		assert.Equal(t, "stopped", lastPayload["phase"])
		assert.Equal(t, "stopped", lastPayload["status"])
		assert.Equal(t, "session ended", lastPayload["message"])
	})
}

func TestClient_Heartbeat(t *testing.T) {
	var lastStatus StatusUpdate

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&lastStatus)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClientWithConfig(server.URL, "test-token", "agent-123")
	ctx := context.Background()

	err := client.Heartbeat(ctx)
	require.NoError(t, err)
	assert.Equal(t, state.Phase(""), lastStatus.Phase)
	assert.Equal(t, state.Activity(""), lastStatus.Activity)
	assert.True(t, lastStatus.Heartbeat)
}

func TestIsHostedMode(t *testing.T) {
	origMode := os.Getenv(EnvAgentMode)
	defer os.Setenv(EnvAgentMode, origMode)

	t.Run("not hosted mode", func(t *testing.T) {
		os.Unsetenv(EnvAgentMode)
		assert.False(t, IsHostedMode())

		os.Setenv(EnvAgentMode, "solo")
		assert.False(t, IsHostedMode())
	})

	t.Run("hosted mode", func(t *testing.T) {
		os.Setenv(EnvAgentMode, "hosted")
		assert.True(t, IsHostedMode())
	})
}

func TestGetAgentID(t *testing.T) {
	origID := os.Getenv(EnvAgentID)
	defer os.Setenv(EnvAgentID, origID)

	os.Setenv(EnvAgentID, "test-agent-id")
	assert.Equal(t, "test-agent-id", GetAgentID())

	os.Unsetenv(EnvAgentID)
	assert.Equal(t, "", GetAgentID())
}

func TestClient_RetryLogic(t *testing.T) {
	t.Run("retries on 5xx errors", func(t *testing.T) {
		attempts := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attempts++
			if attempts < 3 {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("server error"))
				return
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client := NewClientWithConfig(server.URL, "test-token", "agent-123")
		// Use shorter delays for testing
		client.retryBaseDelay = 10 * time.Millisecond
		client.retryMaxDelay = 50 * time.Millisecond

		err := client.UpdateStatus(context.Background(), StatusUpdate{
			Activity: state.ActivityIdle,
			Status:   "idle",
		})
		require.NoError(t, err)
		assert.Equal(t, 3, attempts, "should have retried until success")
	})

	t.Run("does not retry on 4xx errors", func(t *testing.T) {
		attempts := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attempts++
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("bad request"))
		}))
		defer server.Close()

		client := NewClientWithConfig(server.URL, "test-token", "agent-123")
		client.retryBaseDelay = 10 * time.Millisecond

		err := client.UpdateStatus(context.Background(), StatusUpdate{
			Activity: state.ActivityIdle,
			Status:   "idle",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "400")
		assert.Equal(t, 1, attempts, "should not retry on 4xx errors")
	})

	t.Run("gives up after max retries", func(t *testing.T) {
		attempts := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attempts++
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("server error"))
		}))
		defer server.Close()

		client := NewClientWithConfig(server.URL, "test-token", "agent-123")
		client.maxRetries = 2
		client.retryBaseDelay = 10 * time.Millisecond

		err := client.UpdateStatus(context.Background(), StatusUpdate{
			Activity: state.ActivityIdle,
			Status:   "idle",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "3 attempts")
		assert.Equal(t, 3, attempts, "should have attempted 1 + 2 retries")
	})

	t.Run("respects context cancellation during retry", func(t *testing.T) {
		attempts := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attempts++
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		client := NewClientWithConfig(server.URL, "test-token", "agent-123")
		client.maxRetries = 5
		client.retryBaseDelay = 100 * time.Millisecond

		ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
		defer cancel()

		err := client.UpdateStatus(ctx, StatusUpdate{
			Activity: state.ActivityIdle,
			Status:   "idle",
		})
		require.Error(t, err)
		assert.True(t, attempts < 5, "should have stopped early due to context timeout")
	})
}

func TestClient_CalculateBackoff(t *testing.T) {
	client := &Client{
		retryBaseDelay: 100 * time.Millisecond,
		retryMaxDelay:  5 * time.Second,
	}

	// attempt 1: base delay
	assert.Equal(t, 100*time.Millisecond, client.calculateBackoff(1))
	// attempt 2: base * 2
	assert.Equal(t, 200*time.Millisecond, client.calculateBackoff(2))
	// attempt 3: base * 4
	assert.Equal(t, 400*time.Millisecond, client.calculateBackoff(3))
	// attempt 4: base * 8
	assert.Equal(t, 800*time.Millisecond, client.calculateBackoff(4))
}

func TestClient_CalculateBackoff_MaxDelay(t *testing.T) {
	client := &Client{
		retryBaseDelay: 1 * time.Second,
		retryMaxDelay:  3 * time.Second,
	}

	// attempt 1: 1s
	assert.Equal(t, 1*time.Second, client.calculateBackoff(1))
	// attempt 2: 2s
	assert.Equal(t, 2*time.Second, client.calculateBackoff(2))
	// attempt 3: would be 4s, but capped at max
	assert.Equal(t, 3*time.Second, client.calculateBackoff(3))
	// attempt 4: still capped at max
	assert.Equal(t, 3*time.Second, client.calculateBackoff(4))
}

func TestClient_StartHeartbeat(t *testing.T) {
	t.Run("sends heartbeats at interval", func(t *testing.T) {
		heartbeatCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			heartbeatCount++
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client := NewClientWithConfig(server.URL, "test-token", "agent-123")

		ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
		defer cancel()

		done := client.StartHeartbeat(ctx, &HeartbeatConfig{
			Interval: 50 * time.Millisecond,
			Timeout:  100 * time.Millisecond,
		})

		<-done // Wait for heartbeat loop to finish
		// With 250ms timeout and 50ms interval, we expect ~4-5 heartbeats
		assert.GreaterOrEqual(t, heartbeatCount, 3, "should have sent multiple heartbeats")
	})

	t.Run("calls OnError callback on failure", func(t *testing.T) {
		errorCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		client := NewClientWithConfig(server.URL, "test-token", "agent-123")
		// Reduce retries for faster test
		client.maxRetries = 0
		client.retryBaseDelay = 5 * time.Millisecond

		ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
		defer cancel()

		done := client.StartHeartbeat(ctx, &HeartbeatConfig{
			Interval: 50 * time.Millisecond,
			Timeout:  100 * time.Millisecond,
			OnError: func(err error) {
				errorCount++
			},
		})

		<-done
		assert.GreaterOrEqual(t, errorCount, 1, "should have called OnError")
	})

	t.Run("calls OnSuccess callback on success", func(t *testing.T) {
		successCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client := NewClientWithConfig(server.URL, "test-token", "agent-123")

		ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
		defer cancel()

		done := client.StartHeartbeat(ctx, &HeartbeatConfig{
			Interval: 50 * time.Millisecond,
			Timeout:  100 * time.Millisecond,
			OnSuccess: func() {
				successCount++
			},
		})

		<-done
		assert.GreaterOrEqual(t, successCount, 1, "should have called OnSuccess")
	})

	t.Run("stops when context is cancelled", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client := NewClientWithConfig(server.URL, "test-token", "agent-123")

		ctx, cancel := context.WithCancel(context.Background())
		done := client.StartHeartbeat(ctx, &HeartbeatConfig{
			Interval: 1 * time.Second, // Long interval
			Timeout:  100 * time.Millisecond,
		})

		// Cancel immediately
		cancel()

		// Should exit quickly
		select {
		case <-done:
			// Good - loop exited
		case <-time.After(100 * time.Millisecond):
			t.Fatal("heartbeat loop did not exit after context cancellation")
		}
	})
}

func TestParseTokenExpiry(t *testing.T) {
	t.Run("valid JWT with exp claim", func(t *testing.T) {
		// Construct a simple JWT with known expiry
		// Header: {"alg":"HS256","typ":"JWT"}
		// Payload: {"sub":"agent-123","exp":1893456000} (2030-01-01T00:00:00Z)
		// Note: signature doesn't matter for expiry parsing
		token := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJhZ2VudC0xMjMiLCJleHAiOjE4OTM0NTYwMDB9.invalid-sig"

		expiry, err := ParseTokenExpiry(token)
		require.NoError(t, err)
		expected := time.Unix(1893456000, 0)
		assert.Equal(t, expected, expiry)
	})

	t.Run("invalid JWT format", func(t *testing.T) {
		_, err := ParseTokenExpiry("not-a-jwt")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid JWT format")
	})

	t.Run("empty token", func(t *testing.T) {
		_, err := ParseTokenExpiry("")
		assert.Error(t, err)
	})

	t.Run("no exp claim", func(t *testing.T) {
		// Header: {"alg":"HS256","typ":"JWT"}
		// Payload: {"sub":"agent-123"} (no exp)
		token := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJhZ2VudC0xMjMifQ.invalid-sig"

		_, err := ParseTokenExpiry(token)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no expiry claim")
	})
}

func TestClient_RefreshToken(t *testing.T) {
	t.Run("successful refresh", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Equal(t, "/api/v1/agents/agent-123/token/refresh", r.URL.Path)
			assert.Equal(t, "old-token", r.Header.Get("X-Scion-Agent-Token"))

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"token":"new-token","expires_at":"2030-01-01T00:00:00Z"}`))
		}))
		defer server.Close()

		client := NewClientWithConfig(server.URL, "old-token", "agent-123")

		newToken, expiresAt, err := client.RefreshToken(context.Background())
		require.NoError(t, err)
		assert.Equal(t, "new-token", newToken)
		assert.Equal(t, time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC), expiresAt)

		// Client token should be updated
		assert.Equal(t, "new-token", client.GetToken())
	})

	t.Run("server rejects refresh", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("token expired"))
		}))
		defer server.Close()

		client := NewClientWithConfig(server.URL, "expired-token", "agent-123")

		_, _, err := client.RefreshToken(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "401")

		// Client token should not be updated on failure
		assert.Equal(t, "expired-token", client.GetToken())
	})

	t.Run("not configured", func(t *testing.T) {
		client := &Client{}
		_, _, err := client.RefreshToken(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not configured")
	})
}

func TestClient_GetToken(t *testing.T) {
	t.Run("nil client", func(t *testing.T) {
		var c *Client
		assert.Equal(t, "", c.GetToken())
	})

	t.Run("configured client", func(t *testing.T) {
		c := NewClientWithConfig("http://hub", "my-token", "agent-1")
		assert.Equal(t, "my-token", c.GetToken())
	})
}

func TestClient_StartTokenRefresh(t *testing.T) {
	t.Run("refreshes token at scheduled time", func(t *testing.T) {
		refreshed := false
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			refreshed = true
			futureExpiry := time.Now().Add(10 * time.Hour).UTC().Format(time.RFC3339)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"token":"refreshed-token","expires_at":"` + futureExpiry + `"}`))
		}))
		defer server.Close()

		client := NewClientWithConfig(server.URL, "old-token", "agent-123")

		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()

		done := client.StartTokenRefresh(ctx, &TokenRefreshConfig{
			RefreshAt: time.Now(), // Refresh immediately
			Timeout:   5 * time.Second,
			OnRefreshed: func(newExpiry time.Time) {
				assert.True(t, newExpiry.After(time.Now()))
			},
		})

		<-done
		assert.True(t, refreshed, "should have refreshed the token")
		assert.Equal(t, "refreshed-token", client.GetToken())
	})

	t.Run("stops when context is cancelled", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"token":"new","expires_at":"2030-01-01T00:00:00Z"}`))
		}))
		defer server.Close()

		client := NewClientWithConfig(server.URL, "token", "agent-123")

		ctx, cancel := context.WithCancel(context.Background())
		done := client.StartTokenRefresh(ctx, &TokenRefreshConfig{
			RefreshAt: time.Now().Add(1 * time.Hour), // Far in future
		})

		cancel()

		select {
		case <-done:
			// Good
		case <-time.After(200 * time.Millisecond):
			t.Fatal("token refresh loop did not exit after context cancellation")
		}
	})
}

func TestOperatingMode(t *testing.T) {
	// Save and restore env vars
	origEndpoint := os.Getenv(EnvHubEndpoint)
	origURL := os.Getenv(EnvHubURL)
	origMode := os.Getenv(EnvAgentMode)
	defer func() {
		os.Setenv(EnvHubEndpoint, origEndpoint)
		os.Setenv(EnvHubURL, origURL)
		os.Setenv(EnvAgentMode, origMode)
	}()

	tests := []struct {
		name         string
		endpoint     string
		hubURL       string
		agentMode    string
		expectedMode Mode
		expectedStr  string
	}{
		{
			name:         "no hub configured returns ModeLocal",
			endpoint:     "",
			hubURL:       "",
			agentMode:    "",
			expectedMode: ModeLocal,
			expectedStr:  "local",
		},
		{
			name:         "hub endpoint set without hosted mode returns ModeHubConnected",
			endpoint:     "http://hub.example.com",
			hubURL:       "",
			agentMode:    "",
			expectedMode: ModeHubConnected,
			expectedStr:  "hub-connected",
		},
		{
			name:         "hub endpoint set with hosted mode returns ModeHosted",
			endpoint:     "http://hub.example.com",
			hubURL:       "",
			agentMode:    "hosted",
			expectedMode: ModeHosted,
			expectedStr:  "hosted",
		},
		{
			name:         "legacy hub URL set without hosted mode returns ModeHubConnected",
			endpoint:     "",
			hubURL:       "http://hub.example.com",
			agentMode:    "",
			expectedMode: ModeHubConnected,
			expectedStr:  "hub-connected",
		},
		{
			name:         "legacy hub URL set with hosted mode returns ModeHosted",
			endpoint:     "",
			hubURL:       "http://hub.example.com",
			agentMode:    "hosted",
			expectedMode: ModeHosted,
			expectedStr:  "hosted",
		},
		{
			name:         "non-hosted agent mode with hub returns ModeHubConnected",
			endpoint:     "http://hub.example.com",
			hubURL:       "",
			agentMode:    "solo",
			expectedMode: ModeHubConnected,
			expectedStr:  "hub-connected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Unsetenv(EnvHubEndpoint)
			os.Unsetenv(EnvHubURL)
			os.Unsetenv(EnvAgentMode)
			if tt.endpoint != "" {
				os.Setenv(EnvHubEndpoint, tt.endpoint)
			}
			if tt.hubURL != "" {
				os.Setenv(EnvHubURL, tt.hubURL)
			}
			if tt.agentMode != "" {
				os.Setenv(EnvAgentMode, tt.agentMode)
			}

			mode := OperatingMode()
			assert.Equal(t, tt.expectedMode, mode)
			assert.Equal(t, tt.expectedStr, mode.String())
		})
	}
}

func TestOperatingMode_Defaults(t *testing.T) {
	// Save and restore env vars
	origEndpoint := os.Getenv(EnvHubEndpoint)
	origURL := os.Getenv(EnvHubURL)
	origMode := os.Getenv(EnvAgentMode)
	defer func() {
		os.Setenv(EnvHubEndpoint, origEndpoint)
		os.Setenv(EnvHubURL, origURL)
		os.Setenv(EnvAgentMode, origMode)
	}()

	// Clear all relevant env vars
	os.Unsetenv(EnvHubEndpoint)
	os.Unsetenv(EnvHubURL)
	os.Unsetenv(EnvAgentMode)

	mode := OperatingMode()
	assert.Equal(t, ModeLocal, mode, "should default to ModeLocal when no env vars are set")
	assert.Equal(t, "local", mode.String())
}

func TestClient_StartHeartbeat_DefaultConfig(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClientWithConfig(server.URL, "test-token", "agent-123")

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Should work with nil config (uses defaults)
	done := client.StartHeartbeat(ctx, nil)
	<-done
}
