package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
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
	Use:   "run",
	Short: "Runs your project locally",
	Long:  ``,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return errors.New("Please provide a golang file path")
		}

		golangFilePath := args[0]

		goRunCmdOutput, err := exec.Command("go", "run", golangFilePath).CombinedOutput()
		if err != nil {
			return err
		}
		infraDefinitionJsonFilePath := getLastNonEmptyLine(string(goRunCmdOutput))

		run(infraDefinitionJsonFilePath)
		return nil
	},
}

func getLastNonEmptyLine(s string) string {
	lines := strings.Split(s, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) != "" {
			return lines[i]
		}
	}
	return ""
}
