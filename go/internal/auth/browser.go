package auth

import (
	"os/exec"
	"runtime"

	"github.com/TestaVivaDK/plannner-connector/internal/logger"
)

func OpenBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		logger.Log.Info("could not open browser", "error", err)
		return
	}
	go cmd.Wait()
}
