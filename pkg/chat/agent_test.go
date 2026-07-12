package chat

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAgentInstructionsDescribeExistingWorkspace(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	dir := t.TempDir()
	cmd := exec.Command("git", "init", "--initial-branch=feature", dir)
	r.NoError(cmd.Run())

	got := agentInstructions(dir)
	a.Contains(got, "existing local workspace")
	a.Contains(got, "Do not create, switch, rename, or reset git branches")
	a.Contains(got, "AGENTS.md")
	a.Contains(got, "Current working directory: "+dir)
	a.Contains(got, "Current git branch: feature")
}
