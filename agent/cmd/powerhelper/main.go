package main

import (
	"log"
	"os"

	"github.com/Massnaev/jinay-server-panel/agent/internal/powercontrol"
)

func main() {
	if os.Getenv("SERVERPANEL_ENABLE_POWER_ACTIONS") != "true" {
		log.Fatal("power profile control is disabled by configuration")
	}
	socketPath := os.Getenv("SERVERPANEL_POWER_HELPER_SOCKET")
	if socketPath == "" {
		socketPath = powercontrol.DefaultSocketPath
	}
	log.Printf("Jinay power helper listening on %s", socketPath)
	if err := powercontrol.Serve(socketPath, &powercontrol.Manager{}); err != nil {
		log.Fatal(err)
	}
}
