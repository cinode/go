/*
Copyright © 2022 Bartłomiej Święcki (byo)

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

package static_datastore

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
func rootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "static_datastore",
		Short: "Sample application to operate on static datastore",
		Long: `static_datastore can be used to create a simple http server serving
content from datastore served from encrypted datastore layer.

The first step is to generate datastore content with the 'compile'
command which can then be served using the 'server' command.

Note that this tool is supposed to be used for testing purposes only.
It does not guarantee secrecy since the encryption key for the root
node is stored in a plaintext in a file called 'entrypoint.txt'.
`,
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}

	cmd.AddCommand(compileCmd())
	cmd.AddCommand(serverCmd())

	return cmd
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd().Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
