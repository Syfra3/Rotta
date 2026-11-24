package installer

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"
)

// CheckPiDefaultAvailability checks catalog presence, not credentials or provider access.
func CheckPiDefaultAvailability() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "pi", "--offline", "--list-models")
	var out catalogBuffer
	cmd.Stdout = &out
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Pi offline catalog unavailable")
	}
	if out.overflow {
		return fmt.Errorf("Pi offline catalog exceeded output limit")
	}
	var ids []string
	for _, line := range strings.Split(out.String(), "\n") {
		f := strings.Fields(line)
		if len(f) >= 4 {
			ids = append(ids, f[0]+"/"+f[1])
		}
	}
	if !PiDefaultsInCatalog(ids) {
		return fmt.Errorf("pinned Pi defaults are not all present in the offline catalog")
	}
	return nil
}

// PiDefaultsInCatalog reports catalog presence only; it does not verify access.
func PiDefaultsInCatalog(ids []string) bool {
	present := make(map[string]bool, len(ids))
	for _, id := range ids {
		present[id] = true
	}
	if !present[DefaultPiOrchestratorModel()] {
		return false
	}
	for _, id := range DefaultPiRouting() {
		if !present[id] {
			return false
		}
	}
	return true
}

type catalogBuffer struct {
	bytes.Buffer
	overflow bool
}

func (b *catalogBuffer) Write(p []byte) (int, error) {
	n := len(p)
	remaining := (1 << 20) - b.Len()
	if len(p) > remaining {
		b.overflow = true
		p = p[:remaining]
	}
	_, _ = b.Buffer.Write(p)
	return n, nil
}
