package probes

import (
	"fmt"
	"net"
	"time"
)

// TCPProber implements the TCP health check probe
type TCPProber struct{}

// Execute implements the Prober interface for TCP probes
func (p *TCPProber) Execute(ctx *ProbeContext) ProbeResult {
	start := time.Now()

	// Attempt to establish TCP connection
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", ctx.ProbeConfig.Port), 5*time.Second)
	if err != nil {
		return ProbeResult{
			Success:  false,
			Message:  fmt.Sprintf("TCP health check failed: %v", err),
			Duration: time.Since(start),
		}
	}
	defer conn.Close()

	return ProbeResult{
		Success:  true,
		Message:  "TCP health check succeeded",
		Duration: time.Since(start),
	}
}
