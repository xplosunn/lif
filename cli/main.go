package main

import (
	"errors"
	"fmt"
	"github.com/spf13/cobra"
	"os"
)

func main() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(runCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "tenecs",
	Short: "Utilities of the Tenecs programming language",
	Long:  ``,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("0.0.1-alpha")
	},
}

var runCmd = &cobra.Command{
	Use:   "run [FILE]",
	Short: "Run the code",
	Long:  ``,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return errors.New("Please provide a file")
		}

		filePath := args[0]
		run(filePath)
		return nil
	},
}
