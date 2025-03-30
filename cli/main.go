package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/xplosunn/lif/server"
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
	Use:   "lif",
	Short: "Utilities of the Lif library",
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

var (
	serverPort int
)

func init() {
	runCmd.Flags().IntVarP(&serverPort, "port", "p", 8085, "Port for the infrastructure server")
}

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Runs your project locally",
	Long:  ``,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return errors.New("Please provide a golang file path")
		}

		go startServer(serverPort)
		
		// Give the server a moment to start
		time.Sleep(500 * time.Millisecond)
		
		golangFilePath := args[0]

		goRunCmdOutput, err := exec.Command("go", "run", golangFilePath).CombinedOutput()
		if err != nil {
			return err
		}

		infraDefinitionJsonFilePath := getLastNonEmptyLine(string(goRunCmdOutput))
		fmt.Println("goRunCmdOutput:", string(goRunCmdOutput))

		// Send the JSON to the server instead of processing it locally
		jsonBytes, err := os.ReadFile(infraDefinitionJsonFilePath)
		if err != nil {
			return fmt.Errorf("failed to read JSON file: %v", err)
		}
		
		return sendToServer(jsonBytes, serverPort)
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

// startServer starts the infrastructure server on the specified port
func startServer(port int) {
	s := server.NewServer(port)
	if err := s.Start(); err != nil {
		fmt.Printf("Error starting server: %v\n", err)
	}
}

// sendToServer sends the JSON infrastructure definition to the server
func sendToServer(jsonBytes []byte, port int) error {
	url := fmt.Sprintf("http://localhost:%d/deploy", port)
	
	// Create a new HTTP request
	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}
	
	// Set the content type
	req.Header.Set("Content-Type", "application/json")
	
	// Create an HTTP client with a timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	
	// Send the request
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %v", err)
	}
	defer resp.Body.Close()
	
	// Read the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read server response: %v", err)
	}
	
	// Check the status code
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server responded with error (%d): %s", resp.StatusCode, string(body))
	}
	
	// Print the response
	fmt.Println(string(body))
	return nil
}
