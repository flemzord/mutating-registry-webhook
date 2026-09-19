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

package v1alpha1

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRegistryRewriteRuleStatusSerializesReadyFalse(t *testing.T) {
	t.Parallel()

	encoded, err := json.Marshal(RegistryRewriteRuleStatus{Ready: false})
	if err != nil {
		t.Fatalf("marshal status: %v", err)
	}
	if !strings.Contains(string(encoded), `"ready":false`) {
		t.Fatalf("status does not explicitly expose Ready=false: %s", encoded)
	}
}
