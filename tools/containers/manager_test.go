// tools/containers/manager_test.go
package containers_test

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/oioio-space/oioni/tools/containers"
)

// fakePodman returns a WithCmdFactory that simulates podman subcommands.
// toolLines is what the fake writes after the PID line for tool-launch execs.
func fakePodman(t *testing.T, toolLines []string) containers.Option {
	t.Helper()
	return containers.WithCmdFactory(func(name string, args ...string) *exec.Cmd {
		if len(args) == 0 {
			return exec.Command("true")
		}
		switch args[0] {
		case "pull", "rm":
			return exec.Command("true")
		case "run":
			return exec.Command("true")
		case "exec":
			// args[2] == "kill" → signal delivery
			if len(args) >= 3 && args[2] == "kill" {
				return exec.Command("true")
			}
			// Tool launch: write PID then tool lines
			output := "42\n" + strings.Join(toolLines, "\n")
			if len(toolLines) > 0 {
				output += "\n"
			}
			return exec.Command("sh", "-c", fmt.Sprintf("printf %%s %q", output))
		default:
			return exec.Command("true")
		}
	})
}

func testConfig() containers.Config {
	return containers.Config{
		Image:   "oioni/impacket:arm64",
		Name:    "test-container",
		Network: "host",
		Caps:    []string{"NET_RAW", "NET_ADMIN"},
	}
}

func TestProcManager_ContainerInit(t *testing.T) {
	mgr := containers.NewManager(testConfig(), fakePodman(t, nil))
	defer mgr.Close()

	ctx := context.Background()
	proc, err := mgr.Start(ctx, "test", "echo", []string{"hello"})
	if err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	if proc == nil {
		t.Fatal("Start() returned nil proc")
	}
	proc.Wait()
}
