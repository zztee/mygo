//go:build !js

package mygo

import (
	"bufio"
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/egoist/mygo/internal/wait"
)

func TestCheckedInstanceHelper(t *testing.T) {
	mode := os.Getenv("MYGO_CHECKED_HELPER")
	if mode == "" {
		return
	}
	App.SetName(os.Getenv("MYGO_CHECKED_NAME"))
	App.OnSecondInstance(func(args []string, wd string) {
		if !isMainThread() {
			t.Error("handoff handler did not run on the main thread")
		}
		data, _ := json.Marshal(secondInstanceMessage{Args: args, WorkingDir: wd})
		fmt.Printf("DELIVERED %s\n", data)
	})
	primary, err := App.RequestSingleInstanceLockChecked(os.Getenv("MYGO_CHECKED_ID"))
	if err != nil {
		t.Fatal(err)
	}
	if !primary {
		fmt.Println("FORWARDED")
		return
	}
	defer checkedInstance.owner.close()
	if again, err := App.RequestSingleInstanceLockChecked(os.Getenv("MYGO_CHECKED_ID")); !again || err != nil {
		t.Fatalf("same process lost ownership: %v, %v", again, err)
	}
	if other, err := App.RequestSingleInstanceLockChecked("org.mygo.other"); other || err == nil {
		t.Fatalf("one process claimed a second identity: %v, %v", other, err)
	}
	if mode == "quit" {
		checkedInstanceQuitHelper(t)
		return
	}
	fmt.Println("PRIMARY")
	if mode == "hold" {
		_, _ = io.Copy(io.Discard, os.Stdin)
	}
	checkedInstance.owner.close()
	if again, err := App.RequestSingleInstanceLockChecked(os.Getenv("MYGO_CHECKED_ID")); again || err == nil {
		t.Fatalf("released ownership was reported as held: %v, %v", again, err)
	}
}

type checkedHelper struct {
	cmd   *exec.Cmd
	input io.WriteCloser
	lines chan string
	done  chan struct{}
	err   error
}

func startCheckedHelper(t *testing.T, home, id, name, mode, wd string) *checkedHelper {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestCheckedInstanceHelper$", "--", "a note.md", "quote'\".md")
	cmd.Env = append(os.Environ(), "MYGO_CHECKED_HELPER="+mode, "MYGO_CHECKED_ID="+id,
		"MYGO_CHECKED_NAME="+name, "HOME="+home, "USERPROFILE="+home)
	cmd.Dir = wd
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = cmd.Stdout
	input, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	h := &checkedHelper{cmd: cmd, input: input, lines: make(chan string, 128), done: make(chan struct{})}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			h.lines <- scanner.Text()
		}
		h.err = cmd.Wait()
		close(h.lines)
		close(h.done)
	}()
	t.Cleanup(func() {
		input.Close()
		select {
		case <-h.done:
		default:
			_ = cmd.Process.Kill()
			wait.For(t, "helper exit", h.done)
		}
	})
	return h
}

func (h *checkedHelper) line(t *testing.T) string {
	t.Helper()
	line := wait.For(t, "helper output", h.lines)
	if line == "" {
		t.Fatal("helper exited before reporting ownership")
	}
	return line
}

func TestCheckedInstanceProcessOwnership(t *testing.T) {
	home, wd := checkedTestDir(t), checkedTestDir(t)
	primary := startCheckedHelper(t, home, "org.mygo.checked", "First name", "hold", wd)
	if got := primary.line(t); got != "PRIMARY" {
		t.Fatal(got)
	}
	otherWD := checkedTestDir(t)
	secondary := startCheckedHelper(t, home, "org.mygo.checked", "Renamed app", "once", otherWD)
	if got := secondary.line(t); got != "FORWARDED" {
		t.Fatal(got)
	}
	line := primary.line(t)
	var msg secondInstanceMessage
	if !strings.HasPrefix(line, "DELIVERED ") || json.Unmarshal([]byte(strings.TrimPrefix(line, "DELIVERED ")), &msg) != nil {
		t.Fatal(line)
	}
	canonicalWD, err := filepath.EvalSymlinks(otherWD)
	if err != nil {
		t.Fatal(err)
	}
	if msg.WorkingDir != canonicalWD || !strings.Contains(strings.Join(msg.Args, "|"), "a note.md|quote'\".md") {
		t.Fatalf("handoff changed arguments or directory: %+v", msg)
	}
	wait.For(t, "secondary exit", secondary.done)
	if secondary.err != nil {
		t.Fatal(secondary.err)
	}
	independent := startCheckedHelper(t, home, "org.mygo.different", "First name", "once", wd)
	if got := independent.line(t); got != "PRIMARY" {
		t.Fatal(got)
	}
}

func TestCheckedInstanceCompetitionAndCrashRecovery(t *testing.T) {
	home := checkedTestDir(t)
	const count = 6
	helpers := make([]*checkedHelper, count)
	for i := range helpers {
		helpers[i] = startCheckedHelper(t, home, "org.mygo.race", fmt.Sprintf("Name %d", i), "hold", home)
	}
	var winner *checkedHelper
	for _, helper := range helpers {
		// A primary can deliver other launches before its startup message.
		for {
			line := helper.line(t)
			if strings.HasPrefix(line, "DELIVERED ") {
				continue
			}
			if line == "PRIMARY" {
				if winner != nil {
					t.Fatal("two processes acquired the same identity")
				}
				winner = helper
			} else if line != "FORWARDED" {
				t.Fatal(line)
			}
			break
		}
	}
	if winner == nil {
		t.Fatal("no process acquired ownership")
	}
	lockPath, endpointPath := checkedInstancePaths(filepath.Join(home, ".mygo", "instances"), "org.mygo.race")
	before, err := os.Stat(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := winner.cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	wait.For(t, "crashed owner exit", winner.done)
	if _, err := os.Stat(endpointPath); err != nil {
		t.Fatalf("crash fixture did not leave endpoint metadata: %v", err)
	}
	recovered := startCheckedHelper(t, home, "org.mygo.race", "Recovered", "once", home)
	if got := recovered.line(t); got != "PRIMARY" {
		t.Fatal(got)
	}
	wait.For(t, "recovered owner exit", recovered.done)
	if recovered.err != nil {
		t.Fatal(recovered.err)
	}
	after, err := os.Stat(lockPath)
	if err != nil || !os.SameFile(before, after) {
		t.Fatal("recovery replaced the lock file")
	}
}

func checkedTestRequest(t *testing.T, dir string, listen func() (net.Listener, error), dispatch func(context.Context, secondInstanceMessage) bool) (*checkedInstanceOwner, error) {
	t.Helper()
	return requestCheckedInstance(wait.Context(t), dir, "org.mygo.unit", secondInstanceMessage{Args: []string{"test.md"}, WorkingDir: dir}, listen, dispatch)
}

func TestCheckedInstanceFailuresDoNotGrantOwnership(t *testing.T) {
	t.Run("lock", func(t *testing.T) {
		dir := checkedTestDir(t)
		lockPath, _ := checkedInstancePaths(dir, "org.mygo.unit")
		if err := os.Mkdir(lockPath, 0700); err != nil {
			t.Fatal(err)
		}
		owner, err := checkedTestRequest(t, dir, listenCheckedInstance, nil)
		if owner != nil || err == nil {
			t.Fatal("invalid lock file granted ownership")
		}
	})
	t.Run("listen", func(t *testing.T) {
		dir := checkedTestDir(t)
		failure := errors.New("listener unavailable")
		owner, err := checkedTestRequest(t, dir, func() (net.Listener, error) { return nil, failure }, nil)
		if owner != nil || !errors.Is(err, failure) {
			t.Fatalf("listener failure = %v, %v", owner, err)
		}
		owner, err = checkedTestRequest(t, dir, listenCheckedInstance, nil)
		if owner == nil || err != nil {
			t.Fatalf("failed listener leaked the lock: %v", err)
		}
		owner.close()
	})
	t.Run("publish", func(t *testing.T) {
		dir := checkedTestDir(t)
		_, path := checkedInstancePaths(dir, "org.mygo.unit")
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
		owner, err := checkedTestRequest(t, dir, listenCheckedInstance, nil)
		if owner != nil || err == nil {
			t.Fatal("unpublished listener granted ownership")
		}
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		owner, err = checkedTestRequest(t, dir, listenCheckedInstance, nil)
		if owner == nil || err != nil {
			t.Fatalf("failed publication leaked the lock: %v", err)
		}
		owner.close()
	})
	t.Run("unacknowledged", func(t *testing.T) {
		dir := checkedTestDir(t)
		owner, err := checkedTestRequest(t, dir, listenCheckedInstance, func(context.Context, secondInstanceMessage) bool { return false })
		if err != nil {
			t.Fatal(err)
		}
		defer owner.close()
		secondary, err := checkedTestRequest(t, dir, listenCheckedInstance, nil)
		if secondary != nil || err == nil {
			t.Fatal("unacknowledged handoff granted ownership or success")
		}
		if ready, err := owner.ready(); !ready || err != nil {
			t.Fatalf("failed handoff changed the original ownership: %v", err)
		}
	})
	t.Run("initialization canceled", func(t *testing.T) {
		dir := checkedTestDir(t)
		lockPath, _ := checkedInstancePaths(dir, "org.mygo.unit")
		lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
		if err != nil {
			t.Fatal(err)
		}
		defer lock.Close()
		if ok, err := tryCheckedInstanceLock(lock); !ok || err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		owner, err := requestCheckedInstance(ctx, dir, "org.mygo.unit", secondInstanceMessage{}, listenCheckedInstance, nil)
		if owner != nil || !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled initialization = %v, %v", owner, err)
		}
	})
}

func TestCheckedInstanceRejectsInvalidHandoff(t *testing.T) {
	var delivered atomic.Int32
	owner, err := checkedTestRequest(t, checkedTestDir(t), listenCheckedInstance, func(context.Context, secondInstanceMessage) bool { delivered.Add(1); return true })
	if err != nil {
		t.Fatal(err)
	}
	defer owner.close()
	for _, tc := range []struct{ name, token, arg string }{
		{"wrong token", strings.Repeat("0", 64), "a.md"},
		{"oversized", owner.token, strings.Repeat("x", checkedInstanceLimit)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conn, err := net.Dial("tcp4", owner.listener.Addr().String())
			if err != nil {
				t.Fatal(err)
			}
			if err := forwardCheckedInstance(wait.Context(t), conn, tc.token, secondInstanceMessage{Args: []string{tc.arg}}); err == nil {
				t.Fatal("invalid handoff was acknowledged")
			}
		})
	}
	if delivered.Load() != 0 {
		t.Fatal("invalid request was delivered")
	}
}

func TestCheckedInstanceCloseCancelsQueuedHandoff(t *testing.T) {
	entered, release, dispatching := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var delivered atomic.Int32
	off := App.OnSecondInstance(func([]string, string) { delivered.Add(1) })
	defer off()
	owner, err := checkedTestRequest(t, checkedTestDir(t), listenCheckedInstance, func(ctx context.Context, msg secondInstanceMessage) bool {
		close(dispatching)
		return dispatchCheckedInstance(ctx, msg)
	})
	if err != nil {
		t.Fatal(err)
	}
	defer owner.close()
	postMain(func() { close(entered); <-release })
	wait.For(t, "main thread blocked for handoff fixture", entered)
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
	})
	result := make(chan error, 1)
	go func() {
		conn, err := net.Dial("tcp4", owner.listener.Addr().String())
		if err == nil {
			err = forwardCheckedInstance(wait.Context(t), conn, owner.token, secondInstanceMessage{Args: []string{"late.md"}})
		}
		result <- err
	}()
	wait.For(t, "handoff dispatch", dispatching)
	owner.close()
	close(release)
	if err := wait.For(t, "handoff rejected after quit", result); err == nil {
		t.Fatal("closed owner acknowledged queued work")
	}
	onMain(func() {})
	if delivered.Load() != 0 {
		t.Fatal("closed owner delivered queued work")
	}
}

func TestCheckedInstanceListenerFailureKeepsOwnership(t *testing.T) {
	dir := checkedTestDir(t)
	owner, err := checkedTestRequest(t, dir, listenCheckedInstance, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer owner.close()
	owner.listener.Close()
	wait.Until(t, "listener failure observed", func() bool { ready, _ := owner.ready(); return !ready })
	lockPath, _ := checkedInstancePaths(dir, "org.mygo.unit")
	other, err := os.OpenFile(lockPath, os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	if held, err := tryCheckedInstanceLock(other); held || err != nil {
		t.Fatalf("listener failure released ownership: %v, %v", held, err)
	}
}

func checkedTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestCheckedInstanceWaitsForListenerInitialization(t *testing.T) {
	dir := checkedTestDir(t)
	entered, release := make(chan struct{}), make(chan struct{})
	type result struct {
		owner *checkedInstanceOwner
		err   error
	}
	first, second := make(chan result, 1), make(chan result, 1)
	ctx := wait.Context(t)
	go func() {
		owner, err := requestCheckedInstance(ctx, dir, "org.mygo.unit", secondInstanceMessage{}, func() (net.Listener, error) {
			close(entered)
			<-release
			return listenCheckedInstance()
		}, func(context.Context, secondInstanceMessage) bool { return true })
		first <- result{owner, err}
	}()
	wait.For(t, "first process holding lock before listen", entered)
	lockPath, _ := checkedInstancePaths(dir, "org.mygo.unit")
	other, err := os.OpenFile(lockPath, os.O_RDWR, 0600)
	if err != nil {
		close(release)
		t.Fatal(err)
	}
	held, lockErr := tryCheckedInstanceLock(other)
	other.Close()
	if held || lockErr != nil {
		close(release)
		t.Fatalf("initialization did not hold lock: %v, %v", held, lockErr)
	}
	go func() {
		owner, err := requestCheckedInstance(ctx, dir, "org.mygo.unit", secondInstanceMessage{}, listenCheckedInstance, nil)
		second <- result{owner, err}
	}()
	close(release)
	primary := wait.For(t, "initialized owner", first)
	if primary.owner == nil || primary.err != nil {
		t.Fatalf("initialization = %v", primary.err)
	}
	defer primary.owner.close()
	secondary := wait.For(t, "handoff after initialization", second)
	if secondary.owner != nil || secondary.err != nil {
		t.Fatalf("handoff = %v, %v", secondary.owner, secondary.err)
	}
}

func TestCheckedInstanceCanceledForwardSendsNothing(t *testing.T) {
	ln, err := listenCheckedInstance()
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	received := make(chan []byte, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			received <- nil
			return
		}
		defer conn.Close()
		data, _ := io.ReadAll(conn)
		received <- data
	}()
	conn, err := net.Dial("tcp4", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := forwardCheckedInstance(ctx, conn, strings.Repeat("x", 64), secondInstanceMessage{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled forward = %v", err)
	}
	if data := wait.For(t, "canceled connection closed without data", received); len(data) != 0 {
		t.Fatal("canceled forward sent a message")
	}
}

// Generate runs before TestMain so App.Run takes the actual code-generation
// path without starting the fake event loop, just as the CLI starts an app.
func init() {
	if os.Getenv("MYGO_CHECKED_GENERATE_HELPER") != "1" {
		return
	}
	primary, err := App.RequestSingleInstanceLockChecked(os.Getenv("MYGO_CHECKED_ID"))
	if !primary || err != nil {
		fmt.Printf("GENERATE_ERROR %v %v\n", primary, err)
		os.Exit(24)
	}
	NewEvent[int]("checked:generated")
	if err := App.Run(); err != nil {
		fmt.Println(err)
		os.Exit(25)
	}
	fmt.Println("GENERATED")
	os.Exit(0)
}

func TestCheckedInstanceGenerateDoesNotHandoff(t *testing.T) {
	home := checkedTestDir(t)
	primary := startCheckedHelper(t, home, "org.mygo.generate", "Normal app", "hold", home)
	if got := primary.line(t); got != "PRIMARY" {
		t.Fatal(got)
	}
	client := filepath.Join(checkedTestDir(t), "client.ts")
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	cmd.Env = append(os.Environ(), "MYGO_CHECKED_GENERATE_HELPER=1", "MYGO_GENERATE="+client,
		"MYGO_CHECKED_ID=org.mygo.generate", "HOME="+home, "USERPROFILE="+home)
	out, err := cmd.CombinedOutput()
	if err != nil || strings.TrimSpace(string(out)) != "GENERATED" {
		t.Fatalf("generate = %s, %v", out, err)
	}
	data, err := os.ReadFile(client)
	if err != nil || !strings.Contains(string(data), "checked:generated") {
		t.Fatalf("generated client missing event: %v", err)
	}
	select {
	case line := <-primary.lines:
		t.Fatalf("generation reached the running owner: %s", line)
	default:
	}
}

func checkedInstanceQuitHelper(t *testing.T) {
	App.OnSecondInstance(func([]string, string) { fmt.Println("QUIT_DELIVERED"); os.Exit(26) })
	entered, release := make(chan struct{}), make(chan struct{})
	postMain(func() { close(entered); <-release })
	wait.For(t, "blocked main thread", entered)
	App.Quit()
	result := make(chan bool, 1)
	go func() {
		result <- dispatchCheckedInstance(wait.Context(t), secondInstanceMessage{Args: []string{"late.md"}})
	}()
	wait.Until(t, "quit followed by handoff in the real queue", func() bool {
		loop.mu.Lock()
		defer loop.mu.Unlock()
		return len(loop.queue) == 2
	})
	postMain(func() {
		if !App.quitting {
			fmt.Println("QUIT_NOT_APPROVED")
			os.Exit(27)
		}
		fmt.Println("QUIT_REJECTED")
		os.Exit(0)
	})
	close(release)
	wait.For(t, "quit helper exit", result)
}

func TestCheckedInstanceRealQuitRejectsQueuedHandoff(t *testing.T) {
	home := checkedTestDir(t)
	helper := startCheckedHelper(t, home, "org.mygo.quit", "Quit app", "quit", home)
	if got := helper.line(t); got != "QUIT_REJECTED" {
		t.Fatal(got)
	}
	wait.For(t, "quit helper exit", helper.done)
	if helper.err != nil {
		t.Fatal(helper.err)
	}
}

func TestCheckedInstanceCanceledQuitAcceptsHandoff(t *testing.T) {
	offQuit := App.OnBeforeQuit(func(e *QuitEvent) { e.PreventDefault() })
	defer offQuit()
	if onMainValue(App.prepareQuit) {
		t.Fatal("quit was not canceled")
	}
	var calls atomic.Int32
	off := App.OnSecondInstance(func([]string, string) { calls.Add(1) })
	defer off()
	if !dispatchCheckedInstance(wait.Context(t), secondInstanceMessage{}) || calls.Load() != 1 {
		t.Fatal("canceled quit rejected handoff")
	}
}
