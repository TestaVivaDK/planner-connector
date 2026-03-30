package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/TestaVivaDK/plannner-connector/internal/auth"
	"github.com/TestaVivaDK/plannner-connector/internal/graph"
	"github.com/TestaVivaDK/plannner-connector/internal/logger"
	"github.com/TestaVivaDK/plannner-connector/internal/tools"
	"github.com/mark3labs/mcp-go/server"
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

	am := auth.NewAuthManager(clientID, tenantID)
	am.LoadTokenCache()

	if *doLogin {
		if err := am.AcquireTokenInteractively(); err != nil {
			fmt.Fprintf(os.Stderr, "Login failed: %v\n", err)
			os.Exit(1)
		}
		result, _ := am.TestLogin()
		data, _ := json.Marshal(result)
		fmt.Println(string(data))
		return
	}
	if *doLogout {
		am.Logout()
		fmt.Println(`{"message":"Logged out successfully"}`)
		return
	}
	if *verifyLogin {
		result, _ := am.TestLogin()
		data, _ := json.Marshal(result)
		fmt.Println(string(data))
		return
	}

	gc := graph.NewClient(am)
	s := server.NewMCPServer("PlannerMCP", "2.0.0", server.WithToolCapabilities(false), server.WithRecovery())
	tools.RegisterAll(s, am, gc)

	if err := server.ServeStdio(s); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}
