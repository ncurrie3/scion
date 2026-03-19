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
	"testing"
)

func TestLogsFollowValidation(t *testing.T) {
	// --follow should be rejected in hub mode and non-interactive mode.
	// These are integration-level checks; here we verify the flag exists.
	flags := logsCmd.Flags()
	f := flags.Lookup("follow")
	if f == nil {
		t.Fatal("expected --follow flag to be registered")
	}
	if f.Shorthand != "f" {
		t.Errorf("expected --follow shorthand to be 'f', got %q", f.Shorthand)
	}
}
