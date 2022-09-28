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
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"antrea.io/antrea/pkg/antctl/raw/multicluster/common"
)

var cmdToken *cobra.Command

type tokenOptions struct {
	namespace string
	output    string
	k8sClient client.Client
}

var optionsToken *tokenOptions

var tokenExamples = strings.Trim(`
# Fetch a member token and print them in YAML format.
  $ antctl mc get membertoken cluster-east-token -n antrea-multicluster
# Fetch a member token and save the Secret manifest to a file.
  $ antctl mc get membertoken cluster-east-token -n antrea-multicluster -o token-secret.yml
`, "\n")

func (o *tokenOptions) validateAndComplete(cmd *cobra.Command) error {
	if o.namespace == "" {
		return fmt.Errorf("the Namespace is required")
	}
	var err error

	if o.k8sClient == nil {
		o.k8sClient, err = common.NewClient(cmd)
		if err != nil {
			return err
		}
	}
	return nil
}

func NewTokenCommand() *cobra.Command {
	cmdToken = &cobra.Command{
		Use:     "membertoken",
		Short:   "Fetch a member token in a leader cluster",
		Args:    cobra.MaximumNArgs(1),
		Example: tokenExamples,
		RunE:    runEToken,
	}
	o := &tokenOptions{}
	optionsToken = o
	cmdToken.Flags().StringVarP(&o.namespace, "namespace", "n", "", "Namespace of Token")
	cmdToken.Flags().StringVarP(&o.output, "output", "o", "", "Output file to save the token Secret manifest")

	return cmdToken
}

func runEToken(cmd *cobra.Command, args []string) error {
	if err := optionsToken.validateAndComplete(cmd); err != nil {
		return err
	}

	if len(args) != 1 {
		return fmt.Errorf("exactly one NAME is required, got %d", len(args))
	}

	var err error
	var file *os.File
	if optionsToken.output != "" {
		if file, err = os.OpenFile(optionsToken.output, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600); err != nil {
			return err
		}
		defer file.Close()
	}

	if err = common.GetMemberToken(cmd, optionsToken.k8sClient, args[0], optionsToken.namespace, file); err != nil {
		return err
	}
	return nil
}
