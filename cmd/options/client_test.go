// Copyright Jetstack Ltd. See LICENSE for details.
package options

import (
	"errors"
	"testing"

	"github.com/spf13/cobra"
)

func TestClientExtraOptionsValidate(t *testing.T) {

	type testT struct {
		input    []string
		expError error
	}

	for name, test := range map[string]testT{
		"if no flags are provided, should error": {
			input:    []string{},
			expError: errors.New("no client flag options specified"),
		},

		"if only client flags provided then pass": {
			input:    []string{"--server=foo", "--kubeconfig=bla"},
			expError: nil,
		},

		"if only in cluster config provided then pass": {
			input:    []string{"--in-cluster-config"},
			expError: nil,
		},

		"if only in cluster config provided but set to false and other client flags added then pass": {
			input:    []string{"--in-cluster-config=false", "--context=foo", "--namespace=bla"},
			expError: nil,
		},

		"if both in cluster config and other client flags provided then error": {
			input: []string{"--in-cluster-config",
				"--client-certificate=foo", "--client-key=bla"},
			expError: errors.New("if --in-cluster-config is enabled, no other client flag options my be specified"),
		},
	} {
		t.Run(name, func(t *testing.T) {
			c := NewClientExtraFlags()
			cmd := new(cobra.Command)
			c.AddFlags(cmd.Flags())

			if err := cmd.ParseFlags(test.input); err != nil {
				t.Error(err)
				t.FailNow()
			}

			err := c.Validate(cmd)

			if test.expError == nil {
				if err != nil {
					t.Errorf("expected error to be nil, got=%s", err)
				}

			} else {
				if err == nil || test.expError.Error() != err.Error() {
					t.Errorf("unexpected error, exp=%s got=%s",
						test.expError, err)
				}
			}
		})
	}
}
