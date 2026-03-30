package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/TestaVivaDK/plannner-connector/internal/logger"
)

func main() {
	verbose := flag.Bool("verbose", false, "Enable verbose logging to stderr")
	doLogin := flag.Bool("login", false, "Login and exit")
	doLogout := flag.Bool("logout", false, "Logout and exit")
	verifyLogin := flag.Bool("verify-login", false, "Verify login and exit")
	flag.Parse()

	logger.Init(*verbose)

	clientID := os.Getenv("PLANNER_MCP_CLIENT_ID")
	tenantID := os.Getenv("PLANNER_MCP_TENANT_ID")
	if clientID == "" || tenantID == "" {
		fmt.Fprintln(os.Stderr, "Missing PLANNER_MCP_CLIENT_ID or PLANNER_MCP_TENANT_ID")
		os.Exit(1)
	}

	// Placeholders — filled in later tasks
	_, _, _ = *doLogin, *doLogout, *verifyLogin
	fmt.Fprintln(os.Stderr, "Server not yet implemented")
}
