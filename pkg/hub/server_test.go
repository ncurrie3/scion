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

//go:build !no_sqlite

package hub

import (
	"context"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/scion/pkg/store/sqlite"
)

func TestServer_PersistentSigningKeys(t *testing.T) {
	// Create an in-memory SQLite store
	s, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test store: %v", err)
	}

	if err := s.Migrate(context.Background()); err != nil {
		t.Fatalf("failed to migrate test store: %v", err)
	}

	cfg := DefaultServerConfig()

	// Create first server
	srv1 := New(cfg, s)
	t.Cleanup(func() { srv1.Shutdown(context.Background()) })
	if srv1.agentTokenService == nil {
		t.Fatal("agentTokenService not initialized in srv1")
	}
	if srv1.userTokenService == nil {
		t.Fatal("userTokenService not initialized in srv1")
	}

	key1 := srv1.agentTokenService.config.SigningKey
	userKey1 := srv1.userTokenService.config.SigningKey

	// Create second server with the same store
	srv2 := New(cfg, s)
	t.Cleanup(func() { srv2.Shutdown(context.Background()) })
	if srv2.agentTokenService == nil {
		t.Fatal("agentTokenService not initialized in srv2")
	}
	if srv2.userTokenService == nil {
		t.Fatal("userTokenService not initialized in srv2")
	}

	key2 := srv2.agentTokenService.config.SigningKey
	userKey2 := srv2.userTokenService.config.SigningKey

	// Check if keys match
	if string(key1) != string(key2) {
		t.Errorf("agent signing keys do not match: %x != %x", key1, key2)
	}
	if string(userKey1) != string(userKey2) {
		t.Errorf("user signing keys do not match: %x != %x", userKey1, userKey2)
	}
}

func TestServer_GenerateAgentToken_DevAuthAutoGrantsScopes(t *testing.T) {
	s, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test store: %v", err)
	}
	if err := s.Migrate(context.Background()); err != nil {
		t.Fatalf("failed to migrate test store: %v", err)
	}

	cfg := DefaultServerConfig()
	cfg.DevAuthToken = "test-dev-token"
	cfg.AgentTokenConfig = AgentTokenConfig{
		SigningKey:    make([]byte, 32),
		TokenDuration: time.Hour,
	}

	srv := New(cfg, s)
	t.Cleanup(func() { srv.Shutdown(context.Background()) })

	// Generate token without any additional scopes
	token, err := srv.GenerateAgentToken("agent-1", "grove-1")
	if err != nil {
		t.Fatalf("GenerateAgentToken failed: %v", err)
	}

	// Validate the token and check scopes
	claims, err := srv.agentTokenService.ValidateAgentToken(token)
	if err != nil {
		t.Fatalf("ValidateAgentToken failed: %v", err)
	}

	if !claims.HasScope(ScopeAgentStatusUpdate) {
		t.Error("expected ScopeAgentStatusUpdate to be present")
	}
	if !claims.HasScope(ScopeAgentCreate) {
		t.Error("expected ScopeAgentCreate to be auto-granted in dev-auth mode")
	}
	if !claims.HasScope(ScopeAgentLifecycle) {
		t.Error("expected ScopeAgentLifecycle to be auto-granted in dev-auth mode")
	}
	if !claims.HasScope(ScopeAgentNotify) {
		t.Error("expected ScopeAgentNotify to be auto-granted in dev-auth mode")
	}
}

func TestServer_GenerateAgentToken_DevAuthDeduplicatesScopes(t *testing.T) {
	s, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test store: %v", err)
	}
	if err := s.Migrate(context.Background()); err != nil {
		t.Fatalf("failed to migrate test store: %v", err)
	}

	cfg := DefaultServerConfig()
	cfg.DevAuthToken = "test-dev-token"
	cfg.AgentTokenConfig = AgentTokenConfig{
		SigningKey:    make([]byte, 32),
		TokenDuration: time.Hour,
	}

	srv := New(cfg, s)
	t.Cleanup(func() { srv.Shutdown(context.Background()) })

	// Generate token with explicit scopes that overlap with auto-granted ones
	token, err := srv.GenerateAgentToken("agent-1", "grove-1",
		ScopeAgentCreate, ScopeAgentLifecycle, ScopeGroveSecretRead)
	if err != nil {
		t.Fatalf("GenerateAgentToken failed: %v", err)
	}

	claims, err := srv.agentTokenService.ValidateAgentToken(token)
	if err != nil {
		t.Fatalf("ValidateAgentToken failed: %v", err)
	}

	// Count occurrences of each scope to verify deduplication
	scopeCounts := make(map[AgentTokenScope]int)
	for _, sc := range claims.Scopes {
		scopeCounts[sc]++
	}

	if scopeCounts[ScopeAgentCreate] != 1 {
		t.Errorf("expected ScopeAgentCreate once, got %d", scopeCounts[ScopeAgentCreate])
	}
	if scopeCounts[ScopeAgentLifecycle] != 1 {
		t.Errorf("expected ScopeAgentLifecycle once, got %d", scopeCounts[ScopeAgentLifecycle])
	}
	if !claims.HasScope(ScopeGroveSecretRead) {
		t.Error("expected ScopeGroveSecretRead to be present from explicit scopes")
	}
}

func TestServer_GenerateAgentToken_NoDevAuthDoesNotAutoGrant(t *testing.T) {
	s, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create test store: %v", err)
	}
	if err := s.Migrate(context.Background()); err != nil {
		t.Fatalf("failed to migrate test store: %v", err)
	}

	cfg := DefaultServerConfig()
	// DevAuthToken is empty - not dev-auth mode
	cfg.AgentTokenConfig = AgentTokenConfig{
		SigningKey:    make([]byte, 32),
		TokenDuration: time.Hour,
	}

	srv := New(cfg, s)
	t.Cleanup(func() { srv.Shutdown(context.Background()) })

	token, err := srv.GenerateAgentToken("agent-1", "grove-1")
	if err != nil {
		t.Fatalf("GenerateAgentToken failed: %v", err)
	}

	claims, err := srv.agentTokenService.ValidateAgentToken(token)
	if err != nil {
		t.Fatalf("ValidateAgentToken failed: %v", err)
	}

	if !claims.HasScope(ScopeAgentStatusUpdate) {
		t.Error("expected ScopeAgentStatusUpdate to be present")
	}
	if claims.HasScope(ScopeAgentCreate) {
		t.Error("expected ScopeAgentCreate NOT to be auto-granted without dev-auth")
	}
	if claims.HasScope(ScopeAgentLifecycle) {
		t.Error("expected ScopeAgentLifecycle NOT to be auto-granted without dev-auth")
	}
}
