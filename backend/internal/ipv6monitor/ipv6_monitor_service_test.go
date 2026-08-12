package ipv6monitor

import (
	"log"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStartStopCycle reproduces the crash from issue #143: stopping a scan
// closed the IPv6 monitor's device channel and cancelled its context
// permanently, so a subsequent start/stop cycle panicked on "close of closed
// channel". Start() must hand each run a fresh channel and context.
func TestStartStopCycle(t *testing.T) {
	logger := log.Default()
	svc := NewIPv6MonitorService(nil, nil, logger)

	require.NoError(t, svc.Start())
	assert.True(t, svc.IsRunning())
	require.NoError(t, svc.Stop())
	assert.False(t, svc.IsRunning())

	// Second cycle must not panic (close of already-closed channel) and must
	// not exit its goroutines immediately (already-cancelled context).
	require.NoError(t, svc.Start())
	assert.True(t, svc.IsRunning())
	require.NoError(t, svc.Stop())
	assert.False(t, svc.IsRunning())
}

// TestStopWhenNotRunning must be a no-op, not a panic.
func TestStopWhenNotRunning(t *testing.T) {
	svc := NewIPv6MonitorService(nil, nil, log.Default())
	require.NoError(t, svc.Stop())
}
