// Copyright 2022 Antrea Authors
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

package get

import (
	"bytes"
	"log"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	mcscheme "antrea.io/antrea/pkg/antctl/raw/multicluster/scheme"
)

func TestGetAccessToken(t *testing.T) {
	secretContent := []byte(`apiVersion: v1
kind: Secret
metadata:
  name: default-member-token
data:
  ca.crt: YWJjZAo=
  namespace: ZGVmYXVsdAo=
  token: YWJjZAo=
type: Opaque`)
	existingSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "default-member-token",
			Annotations: map[string]string{
				"multicluster.antrea.io/created-by-antctl": "true",
			},
		},
		Data: map[string][]byte{"token": secretContent},
	}

	tests := []struct {
		name           string
		namespace      string
		outputfile     string
		expectedOutput string
		secretFile     bool
		failureType    string
		tokeName       string
	}{
		{
			name:           "fetch successfully",
			tokeName:       "default-member-token",
			namespace:      "default",
			expectedOutput: "",
		},
		{
			name:           "fetch successfully with file",
			tokeName:       "default-member-token",
			namespace:      "default",
			expectedOutput: "Member token saved to",
			secretFile:     true,
		},
		{
			name:           "fail to fetch without name",
			namespace:      "default",
			expectedOutput: "exactly one NAME is required, got 0",
		},
		{
			name:           "fail to fetch without namespace",
			namespace:      "",
			expectedOutput: "the Namespace is required",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewTokenCommand()
			buf := new(bytes.Buffer)
			cmd.SetOutput(buf)
			cmd.SetOut(buf)
			cmd.SetErr(buf)

			optionsToken.namespace = tt.namespace
			optionsToken.k8sClient = fake.NewClientBuilder().WithScheme(mcscheme.Scheme).WithObjects(existingSecret).Build()

			if tt.tokeName != "" {
				cmd.SetArgs([]string{tt.tokeName})
			}
			if tt.secretFile {
				secret, err := os.CreateTemp("", "secret")
				if err != nil {
					log.Fatal(err)
				}
				defer os.Remove(secret.Name())
				secret.Write([]byte(secretContent))
				optionsToken.output = secret.Name()
			}
			err := cmd.Execute()
			if err != nil {
				if tt.name == "fetch successfully" {
					assert.Equal(t, err.Error(), tt.expectedOutput)
				} else {
					assert.Contains(t, err.Error(), tt.expectedOutput)
				}
			} else {
				assert.Contains(t, buf.String(), tt.expectedOutput)
			}
		})
	}
}
