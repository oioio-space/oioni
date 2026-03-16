// tools/containers/manager_test.go
package containers_test

import (
	"context"
	"errors"
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
			return exec.Command("sh", "-c", fmt.Sprintf("printf '%%b' %q", output))
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

func TestProcManager_StartAndLines(t *testing.T) {
	mgr := containers.NewManager(testConfig(), fakePodman(t, []string{"hello", "world"}))
	defer mgr.Close()

	proc, err := mgr.Start(context.Background(), "myproc", "echo", []string{"hello"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	var got []string
	for l := range proc.Lines() {
		got = append(got, l)
	}
	if len(got) != 2 || got[0] != "hello" || got[1] != "world" {
		t.Fatalf("Lines() = %v, want [hello world]", got)
	}
}

func TestProcManager_ErrAlreadyRunning(t *testing.T) {
	// Fake that blocks until killed
	block := make(chan struct{})
	mgr := containers.NewManager(testConfig(), containers.WithCmdFactory(func(name string, args ...string) *exec.Cmd {
		if len(args) > 0 && args[0] == "exec" {
			if len(args) >= 3 && args[2] == "kill" {
				close(block) // kill signal received — only called once
				return exec.Command("true")
			}
			// Tool launch: write PID then block (simulates long-running process)
			return exec.Command("sh", "-c", "echo 42; cat")
		}
		return exec.Command("true") // pull, run, rm — must NOT close(block)
	}))
	defer mgr.Close()

	ctx := context.Background()
	if _, err := mgr.Start(ctx, "same", "cat", nil); err != nil {
		t.Fatalf("first Start: %v", err)
	}
	if _, err := mgr.Start(ctx, "same", "cat", nil); !errors.Is(err, containers.ErrAlreadyRunning) {
		t.Fatalf("want ErrAlreadyRunning, got %v", err)
	}
}

func TestProcManager_ErrManagerClosed(t *testing.T) {
	mgr := containers.NewManager(testConfig(), fakePodman(t, nil))
	mgr.Close()
	_, err := mgr.Start(context.Background(), "x", "echo", nil)
	if !errors.Is(err, containers.ErrManagerClosed) {
		t.Fatalf("want ErrManagerClosed, got %v", err)
	}
}

func TestProcManager_List(t *testing.T) {
	mgr := containers.NewManager(testConfig(), fakePodman(t, []string{"out"}))
	defer mgr.Close()
	ctx := context.Background()

	if _, err := mgr.Start(ctx, "p1", "echo", nil); err != nil {
		t.Fatal(err)
	}

	list := mgr.List()
	if len(list) != 1 || list[0] != "p1" {
		t.Fatalf("List() = %v, want [p1]", list)
	}
}

func TestProcManager_CloseWaitsForWatchers(t *testing.T) {
	mgr := containers.NewManager(testConfig(), fakePodman(t, nil))
	ctx := context.Background()
	if _, err := mgr.Start(ctx, "p", "echo", nil); err != nil {
		t.Fatal(err)
	}
	mgr.Close()
	if list := mgr.List(); len(list) != 0 {
		t.Fatalf("after Close: List() = %v, want empty", list)
	}
}
