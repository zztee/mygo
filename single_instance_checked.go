//go:build !js

package mygo

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

const checkedInstanceLimit = 1 << 20
const checkedInstanceTimeout = 5 * time.Second

var checkedInstance struct {
	sync.Mutex
	identity string
	owner    *checkedInstanceOwner
}

type checkedInstanceEndpoint struct {
	Address string `json:"address"`
	Token   string `json:"token"`
}

type checkedInstanceRequest struct {
	Token   string                `json:"token"`
	Message secondInstanceMessage `json:"message"`
}

type checkedInstanceOwner struct {
	mu        sync.Mutex
	listener  net.Listener
	lock      *os.File
	token     string
	closed    chan struct{}
	ctx       context.Context
	cancel    context.CancelFunc
	acceptErr error
	dispatch  func(context.Context, secondInstanceMessage) bool
}

// RequestSingleInstanceLockChecked obtains exclusive ownership of identity for
// this user, or hands the command line to its owner. Use a fixed reverse-DNS ID;
// the application name, executable path and working directory are not its identity.
// All instances must use this API with the same ID and user home directory. It
// does not coordinate with the legacy RequestSingleInstanceLock protocol.
//
// True means both an OS file lock and a loopback listener are held until OnQuit.
// False with no error means the owner ran its OnSecondInstance handlers and
// delivered the arguments to its open handlers. Every error means startup must
// stop; a failed acknowledgement may still have delivered the arguments, so do
// not retry automatically. Initialization and handoff have a five-second limit.
// Call before Run, after registering handlers. Browser instances are unsupported.
// In MYGO_GENERATE mode it returns true without locking or forwarding: Run only
// writes the TypeScript client and never starts an editable application.
func (a *Application) RequestSingleInstanceLockChecked(identity string) (bool, error) {
	if os.Getenv("MYGO_GENERATE") != "" {
		return true, nil
	}
	if strings.TrimSpace(identity) == "" {
		return false, errors.New("mygo: single-instance identity is empty")
	}
	checkedInstance.Lock()
	defer checkedInstance.Unlock()
	if checkedInstance.owner != nil {
		if checkedInstance.identity != identity {
			return false, errors.New("mygo: this process already owns another identity")
		}
		return checkedInstance.owner.ready()
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return false, fmt.Errorf("mygo: single-instance home: %w", err)
	}
	wd, err := os.Getwd()
	if err != nil {
		return false, fmt.Errorf("mygo: single-instance working directory: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), checkedInstanceTimeout)
	defer cancel()
	owner, err := requestCheckedInstance(ctx, filepath.Join(home, ".mygo", "instances"), identity,
		secondInstanceMessage{Args: os.Args[1:], WorkingDir: wd}, listenCheckedInstance, dispatchCheckedInstance)
	if err != nil || owner == nil {
		return false, err
	}
	checkedInstance.identity, checkedInstance.owner = identity, owner
	a.OnQuit(owner.close)
	return true, nil
}

func checkedInstancePaths(dir, identity string) (lock, endpoint string) {
	sum := sha256.Sum256([]byte(identity))
	base := filepath.Join(dir, hex.EncodeToString(sum[:]))
	return base + ".lock", base + ".json"
}

func listenCheckedInstance() (net.Listener, error) { return net.Listen("tcp4", "127.0.0.1:0") }

func requestCheckedInstance(ctx context.Context, dir, identity string, msg secondInstanceMessage,
	listen func() (net.Listener, error), dispatch func(context.Context, secondInstanceMessage) bool,
) (*checkedInstanceOwner, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("mygo: single-instance directory: %w", err)
	}
	fi, err := os.Lstat(dir)
	if err != nil || !fi.IsDir() || (runtime.GOOS != "windows" && fi.Mode().Perm()&0077 != 0) {
		return nil, fmt.Errorf("mygo: single-instance directory is not private: %s", dir)
	}
	lockPath, endpointPath := checkedInstancePaths(dir, identity)
	if fi, err := os.Lstat(lockPath); err == nil && !fi.Mode().IsRegular() {
		return nil, errors.New("mygo: single-instance lock is not a regular file")
	}
	lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("mygo: single-instance lock: %w", err)
	}
	keep := false
	defer func() {
		if !keep {
			lock.Close()
		}
	}()
	for {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("mygo: single-instance initialization: %w", err)
		}
		locked, err := tryCheckedInstanceLock(lock)
		if err != nil {
			return nil, fmt.Errorf("mygo: single-instance lock: %w", err)
		}
		if locked {
			actual, statErr := lock.Stat()
			current, pathErr := os.Lstat(lockPath)
			if statErr != nil || pathErr != nil || !os.SameFile(actual, current) || !actual.Mode().IsRegular() {
				return nil, errors.New("mygo: single-instance lock file changed")
			}
			owner, err := startCheckedInstance(lock, endpointPath, listen, dispatch)
			keep = err == nil
			return owner, err
		}
		data, err := os.ReadFile(endpointPath)
		if err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("mygo: single-instance endpoint: %w", err)
		}
		var endpoint checkedInstanceEndpoint
		if err == nil && json.Unmarshal(data, &endpoint) == nil && len(endpoint.Token) == 64 {
			host, _, err := net.SplitHostPort(endpoint.Address)
			if err != nil || host != "127.0.0.1" {
				return nil, errors.New("mygo: invalid single-instance endpoint")
			}
			conn, err := (&net.Dialer{}).DialContext(ctx, "tcp4", endpoint.Address)
			if err == nil {
				// Once anything is sent, a failed acknowledgement is ambiguous.
				// Never take ownership or resend after that point.
				return nil, forwardCheckedInstance(ctx, conn, endpoint.Token, msg)
			}
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("mygo: single-instance initialization: %w", ctx.Err())
		case <-time.After(20 * time.Millisecond):
		}
	}
}

func startCheckedInstance(lock *os.File, path string, listen func() (net.Listener, error),
	dispatch func(context.Context, secondInstanceMessage) bool,
) (*checkedInstanceOwner, error) {
	ln, err := listen()
	if err != nil {
		return nil, fmt.Errorf("mygo: single-instance listener: %w", err)
	}
	token := make([]byte, 32)
	if _, err := rand.Read(token); err != nil {
		ln.Close()
		return nil, fmt.Errorf("mygo: single-instance token: %w", err)
	}
	ownerCtx, cancel := context.WithCancel(context.Background())
	owner := &checkedInstanceOwner{ctx: ownerCtx, cancel: cancel, listener: ln, lock: lock, token: hex.EncodeToString(token), closed: make(chan struct{}), dispatch: dispatch}
	data, err := json.Marshal(checkedInstanceEndpoint{Address: ln.Addr().String(), Token: owner.token})
	if err == nil {
		err = writeCheckedInstanceEndpoint(path, data)
	}
	if err != nil {
		ln.Close()
		cancel()
		return nil, fmt.Errorf("mygo: single-instance publish: %w", err)
	}
	go owner.accept()
	return owner, nil
}

func writeCheckedInstanceEndpoint(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".instance-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	_, err = f.Write(data)
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

func forwardCheckedInstance(ctx context.Context, conn net.Conn, token string, msg secondInstanceMessage) error {
	defer conn.Close()
	if err := ctx.Err(); err != nil {
		return err
	}
	deadline, _ := ctx.Deadline()
	if deadline.IsZero() {
		deadline = time.Now().Add(checkedInstanceTimeout)
	}
	if err := conn.SetDeadline(deadline); err != nil {
		return err
	}
	data, err := json.Marshal(checkedInstanceRequest{Token: token, Message: msg})
	if err != nil || len(data) > checkedInstanceLimit {
		return errors.New("mygo: single-instance arguments exceed the message limit")
	}
	if _, err := conn.Write(data); err != nil {
		return fmt.Errorf("mygo: single-instance send: %w", err)
	}
	if err := conn.(*net.TCPConn).CloseWrite(); err != nil {
		return fmt.Errorf("mygo: single-instance send: %w", err)
	}
	ack, err := io.ReadAll(io.LimitReader(conn, 16))
	if err != nil || string(ack) != "accepted\n" {
		return errors.New("mygo: single-instance handoff was not acknowledged")
	}
	return nil
}

func (owner *checkedInstanceOwner) accept() {
	for {
		conn, err := owner.listener.Accept()
		if err != nil {
			owner.mu.Lock()
			owner.acceptErr = err
			owner.mu.Unlock()
			// Keep the lock until quit, even if the listener fails. Existing
			// windows may still be writing and must not gain another owner.
			return
		}
		go owner.receive(conn)
	}
}

func (owner *checkedInstanceOwner) receive(conn net.Conn) {
	defer conn.Close()
	ctx, cancel := context.WithTimeout(owner.ctx, checkedInstanceTimeout)
	defer cancel()
	deadline, _ := ctx.Deadline()
	if conn.SetDeadline(deadline) != nil {
		return
	}
	data, err := io.ReadAll(io.LimitReader(conn, checkedInstanceLimit+1))
	if err != nil || len(data) > checkedInstanceLimit {
		return
	}
	var req checkedInstanceRequest
	if json.Unmarshal(data, &req) != nil || subtle.ConstantTimeCompare([]byte(req.Token), []byte(owner.token)) != 1 {
		return
	}
	select {
	case <-owner.closed:
		return
	default:
	}
	if owner.dispatch(ctx, req.Message) {
		_, _ = io.WriteString(conn, "accepted\n")
	}
}

func dispatchCheckedInstance(ctx context.Context, msg secondInstanceMessage) bool {
	done := make(chan struct{})
	if !postMain(func() {
		if ctx.Err() != nil || App.quitting {
			return
		}
		for _, fn := range singleInstance.handlers.snapshot() {
			fn(msg.Args, msg.WorkingDir)
			if ctx.Err() != nil || App.quitting {
				return
			}
		}
		deliverArgs(msg.Args, msg.WorkingDir)
		close(done)
	}) {
		return false
	}
	select {
	case <-ctx.Done():
		return false
	case <-done:
		return true
	}
}

func (owner *checkedInstanceOwner) ready() (bool, error) {
	owner.mu.Lock()
	defer owner.mu.Unlock()
	select {
	case <-owner.closed:
		return false, errors.New("mygo: single-instance ownership has been released")
	default:
	}
	if owner.acceptErr != nil {
		return false, fmt.Errorf("mygo: single-instance listener stopped: %w", owner.acceptErr)
	}
	return true, nil
}

func (owner *checkedInstanceOwner) close() {
	owner.mu.Lock()
	defer owner.mu.Unlock()
	select {
	case <-owner.closed:
		return
	default:
		close(owner.closed)
		owner.cancel()
	}
	owner.listener.Close()
	// Never delete the lock file: every process must lock the same inode.
	owner.lock.Close()
}
