package tui

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/Syfra3/Rotta/internal/installer"
	tea "github.com/charmbracelet/bubbletea"
)

const modelDiscoveryTimeout = 5 * time.Second
const modelDiscoveryOutputLimit = 1 << 20

type modelDiscoveryResultMsg struct {
	models     []string
	err        string
	generation int
}

var discoverOpenCodeModels = discoverOpenCodeModelsDirect

func discoverOpenCodeModelsCmd(generation int) tea.Cmd {
	return func() tea.Msg {
		models, err := discoverOpenCodeModels()
		if err != nil {
			return modelDiscoveryResultMsg{err: err.Error(), generation: generation}
		}
		if len(models) == 0 {
			return modelDiscoveryResultMsg{err: "OpenCode returned no models. Configure a provider, then rerun the installer.", generation: generation}
		}
		return modelDiscoveryResultMsg{models: models, generation: generation}
	}
}

func discoverOpenCodeModelsDirect() ([]string, error) {
	path, err := exec.LookPath("opencode")
	if err != nil {
		return nil, errors.New("OpenCode CLI is unavailable; install OpenCode or choose Default or Disabled")
	}
	ctx, cancel := context.WithTimeout(context.Background(), modelDiscoveryTimeout)
	defer cancel()
	command := exec.CommandContext(ctx, path, "models")
	var output limitedBuffer
	command.Stdout = &output
	command.Stderr = io.Discard
	if err := command.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, errors.New("OpenCode model discovery timed out; choose Default or Disabled, or try again")
		}
		return nil, errors.New("OpenCode could not list models; check its local configuration and try again")
	}
	return parseOpenCodeModels(output.Bytes()), nil
}

type limitedBuffer struct{ bytes.Buffer }

func (b *limitedBuffer) Write(data []byte) (int, error) {
	originalLength := len(data)
	remaining := modelDiscoveryOutputLimit - b.Len()
	if remaining > 0 {
		if len(data) > remaining {
			data = data[:remaining]
		}
		_, _ = b.Buffer.Write(data)
	}
	return originalLength, nil
}

func parseOpenCodeModels(output []byte) []string {
	unique := map[string]bool{}
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		id := fields[0]
		if installer.IsValidOpenCodeModelID(id) {
			unique[id] = true
		}
	}
	models := make([]string, 0, len(unique))
	for model := range unique {
		models = append(models, model)
	}
	sort.Strings(models)
	return models
}
