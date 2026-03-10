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

package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/GoogleCloudPlatform/scion/pkg/config"
	"github.com/GoogleCloudPlatform/scion/pkg/hubclient"
	"github.com/spf13/cobra"
)

var (
	notificationsShowAll  bool
	notificationsJSON     bool
	notificationsAckAll   bool
)

// hubNotificationsCmd lists notifications for the current user
var hubNotificationsCmd = &cobra.Command{
	Use:     "notifications",
	Aliases: []string{"notification"},
	Short:   "List notifications",
	Long: `List notifications from the Hub for the current user.

By default, only unacknowledged notifications are shown.
Use --all to include previously acknowledged notifications.

Examples:
  # List unacknowledged notifications
  scion hub notifications

  # List all notifications including acknowledged
  scion hub notifications --all

  # Output as JSON
  scion hub notifications --json`,
	RunE: runHubNotifications,
}

// hubNotificationsAckCmd acknowledges notifications
var hubNotificationsAckCmd = &cobra.Command{
	Use:   "ack [notification-id]",
	Short: "Acknowledge notification(s)",
	Long: `Acknowledge one or all notifications.

With an ID argument, acknowledges that specific notification.
With --all flag, acknowledges all unacknowledged notifications.

Examples:
  # Acknowledge a specific notification
  scion hub notifications ack a1b2c3d4

  # Acknowledge all notifications
  scion hub notifications ack --all`,
	RunE: runHubNotificationsAck,
}

func init() {
	hubCmd.AddCommand(hubNotificationsCmd)
	hubNotificationsCmd.AddCommand(hubNotificationsAckCmd)

	hubNotificationsCmd.Flags().BoolVar(&notificationsShowAll, "all", false, "Include acknowledged notifications")
	hubNotificationsCmd.Flags().BoolVar(&notificationsJSON, "json", false, "Output in JSON format")

	hubNotificationsAckCmd.Flags().BoolVar(&notificationsAckAll, "all", false, "Acknowledge all notifications")
}

func runHubNotifications(cmd *cobra.Command, args []string) error {
	if notificationsJSON {
		outputFormat = "json"
	}

	resolvedPath, _, err := config.ResolveGrovePath(grovePath)
	if err != nil {
		return fmt.Errorf("failed to resolve grove path: %w", err)
	}

	settings, err := config.LoadSettings(resolvedPath)
	if err != nil {
		return fmt.Errorf("failed to load settings: %w", err)
	}

	client, err := getHubClient(settings)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	opts := &hubclient.ListNotificationsOptions{
		OnlyUnacknowledged: !notificationsShowAll,
	}

	notifs, err := client.Notifications().List(ctx, opts)
	if err != nil {
		return fmt.Errorf("failed to list notifications: %w", err)
	}

	if isJSONOutput() {
		return outputJSON(notifs)
	}

	if len(notifs) == 0 {
		fmt.Println("No notifications")
		return nil
	}

	fmt.Printf("%-12s  %-14s  %-20s  %-20s  %s\n", "ID", "AGENT", "STATUS", "TIME", "MESSAGE")
	fmt.Printf("%-12s  %-14s  %-20s  %-20s  %s\n", "------------", "--------------", "--------------------", "--------------------", "-------")
	for _, n := range notifs {
		shortID := n.ID
		if len(shortID) > 12 {
			shortID = shortID[:12]
		}
		// Extract agent slug from the notification; use agentId as fallback
		agentDisplay := n.AgentID
		if len(agentDisplay) > 14 {
			agentDisplay = agentDisplay[:11] + "..."
		}
		timeStr := n.CreatedAt.Format("2006-01-02 15:04")
		msg := n.Message
		if len(msg) > 60 {
			msg = msg[:57] + "..."
		}
		fmt.Printf("%-12s  %-14s  %-20s  %-20s  %s\n", shortID, agentDisplay, truncate(n.Status, 20), timeStr, msg)
	}

	return nil
}

func runHubNotificationsAck(cmd *cobra.Command, args []string) error {
	hasID := len(args) > 0

	if !hasID && !notificationsAckAll {
		return fmt.Errorf("provide a notification ID or use --all to acknowledge all notifications")
	}
	if hasID && notificationsAckAll {
		return fmt.Errorf("provide either a notification ID or --all, not both")
	}

	resolvedPath, _, err := config.ResolveGrovePath(grovePath)
	if err != nil {
		return fmt.Errorf("failed to resolve grove path: %w", err)
	}

	settings, err := config.LoadSettings(resolvedPath)
	if err != nil {
		return fmt.Errorf("failed to load settings: %w", err)
	}

	client, err := getHubClient(settings)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if notificationsAckAll {
		if err := client.Notifications().AcknowledgeAll(ctx); err != nil {
			return fmt.Errorf("failed to acknowledge notifications: %w", err)
		}
		fmt.Println("All notifications acknowledged.")
		return nil
	}

	notifID := args[0]
	if err := client.Notifications().Acknowledge(ctx, notifID); err != nil {
		return fmt.Errorf("failed to acknowledge notification: %w", err)
	}
	fmt.Printf("Notification %s acknowledged.\n", notifID)
	return nil
}
