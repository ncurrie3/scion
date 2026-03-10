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

package hubclient

import (
	"context"
	"net/url"
	"time"

	"github.com/GoogleCloudPlatform/scion/pkg/apiclient"
)

// NotificationService handles notification operations.
type NotificationService interface {
	// List returns notifications for the current user.
	List(ctx context.Context, opts *ListNotificationsOptions) ([]Notification, error)

	// Acknowledge marks a single notification as acknowledged.
	Acknowledge(ctx context.Context, id string) error

	// AcknowledgeAll marks all unacknowledged notifications as acknowledged.
	AcknowledgeAll(ctx context.Context) error
}

// notificationService is the implementation of NotificationService.
type notificationService struct {
	c *client
}

// ListNotificationsOptions configures notification listing.
type ListNotificationsOptions struct {
	OnlyUnacknowledged bool
}

// Notification represents a notification from the Hub API.
type Notification struct {
	ID             string    `json:"id"`
	SubscriptionID string    `json:"subscriptionId"`
	AgentID        string    `json:"agentId"`
	GroveID        string    `json:"groveId"`
	SubscriberType string    `json:"subscriberType"`
	SubscriberID   string    `json:"subscriberId"`
	Status         string    `json:"status"`
	Message        string    `json:"message"`
	Dispatched     bool      `json:"dispatched"`
	Acknowledged   bool      `json:"acknowledged"`
	CreatedAt      time.Time `json:"createdAt"`
}

// List returns notifications for the current user.
func (s *notificationService) List(ctx context.Context, opts *ListNotificationsOptions) ([]Notification, error) {
	query := url.Values{}
	if opts != nil && !opts.OnlyUnacknowledged {
		query.Set("acknowledged", "true")
	}

	resp, err := s.c.transport.GetWithQuery(ctx, "/api/v1/notifications", query, nil)
	if err != nil {
		return nil, err
	}
	result, err := apiclient.DecodeResponse[[]Notification](resp)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return []Notification{}, nil
	}
	return *result, nil
}

// Acknowledge marks a single notification as acknowledged.
func (s *notificationService) Acknowledge(ctx context.Context, id string) error {
	resp, err := s.c.transport.Post(ctx, "/api/v1/notifications/"+url.PathEscape(id)+"/ack", nil, nil)
	if err != nil {
		return err
	}
	return apiclient.CheckResponse(resp)
}

// AcknowledgeAll marks all unacknowledged notifications as acknowledged.
func (s *notificationService) AcknowledgeAll(ctx context.Context) error {
	resp, err := s.c.transport.Post(ctx, "/api/v1/notifications/ack-all", nil, nil)
	if err != nil {
		return err
	}
	return apiclient.CheckResponse(resp)
}
