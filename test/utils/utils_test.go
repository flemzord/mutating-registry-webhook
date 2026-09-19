/*
Copyright 2025 flemzord.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package utils

import (
	"strings"
	"testing"
)

func TestRedactCommandBearerToken(t *testing.T) {
	t.Parallel()

	const token = "eyJhbGciOiJSUzI1NiJ9.payload_signature-with~chars"
	command := "curl -H 'Authorization: Bearer " + token + "' https://example.test/metrics"

	redacted := redactCommand(command)
	if strings.Contains(redacted, token) {
		t.Fatalf("redacted command still contains bearer token: %q", redacted)
	}
	if !strings.Contains(redacted, "Authorization: Bearer [REDACTED]") {
		t.Fatalf("redacted command does not preserve a useful authorization marker: %q", redacted)
	}
}
