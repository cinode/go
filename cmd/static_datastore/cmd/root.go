package cmd

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
