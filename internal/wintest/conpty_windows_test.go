package wintest

import (
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
	"unsafe"

	"github.com/linhh-phv/claude-acc/internal/secret"
	"golang.org/x/sys/windows"
)

// A Windows pseudo-console (ConPTY): the program sees a real console — the
// hidden token prompt, the [y/N] question and the login code prompt all
// read it — and the test types into it once a prompt appears.

var (
	kernel32                      = windows.NewLazySystemDLL("kernel32.dll")
	procUpdateProcThreadAttribute = kernel32.NewProc("UpdateProcThreadAttribute")
)

type console struct {
	t    *testing.T
	hpc  windows.Handle
	in   *os.File
	out  *os.File
	proc windows.Handle
	job  windows.Handle
	mu   sync.Mutex
	buf  []byte
	done chan struct{}
	seen int // offset in plain() already matched
}

func startConsole(t *testing.T, exe string, args, env []string, dir string) *console {
	t.Helper()
	var inR, inW, outR, outW windows.Handle
	if err := windows.CreatePipe(&inR, &inW, nil, 0); err != nil {
		t.Fatal(err)
	}
	if err := windows.CreatePipe(&outR, &outW, nil, 0); err != nil {
		t.Fatal(err)
	}
	var hpc windows.Handle
	if err := windows.CreatePseudoConsole(windows.Coord{X: 220, Y: 60}, inR, outW, 0, &hpc); err != nil {
		t.Fatalf("CreatePseudoConsole: %v (needs Windows 10 1809 or later)", err)
	}
	windows.CloseHandle(inR)
	windows.CloseHandle(outW)

	attr, err := windows.NewProcThreadAttributeList(1)
	if err != nil {
		t.Fatal(err)
	}
	defer attr.Delete()
	// The attribute VALUE is the console handle itself, not a pointer to it.
	if r, _, err := procUpdateProcThreadAttribute.Call(uintptr(unsafe.Pointer(attr.List())), 0,
		windows.PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE, uintptr(hpc), unsafe.Sizeof(hpc), 0, 0); r == 0 {
		t.Fatalf("UpdateProcThreadAttribute: %v", err)
	}

	var si windows.StartupInfoEx
	si.Cb = uint32(unsafe.Sizeof(si))
	// No handles given: the child must use the pseudo-console, not whatever
	// this test process's own stdio happens to be.
	si.Flags = windows.STARTF_USESTDHANDLES
	si.ProcThreadAttributeList = attr.List()

	cmdline, _ := windows.UTF16PtrFromString(windows.ComposeCommandLine(append([]string{exe}, args...)))
	dirp, _ := windows.UTF16PtrFromString(dir)
	block := envBlock(env)
	var pi windows.ProcessInformation
	flags := uint32(windows.EXTENDED_STARTUPINFO_PRESENT | windows.CREATE_UNICODE_ENVIRONMENT | windows.CREATE_SUSPENDED)
	if err := windows.CreateProcess(nil, cmdline, nil, nil, false, flags, &block[0], dirp, &si.StartupInfo, &pi); err != nil {
		t.Fatalf("CreateProcess %s: %v", exe, err)
	}

	// A job object takes the whole process tree down with it, so a hung run
	// never leaves claude or claude-acc behind.
	job, err := windows.CreateJobObject(nil, nil)
	if err == nil {
		info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
			BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE},
		}
		windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation,
			uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)))
		windows.AssignProcessToJobObject(job, pi.Process)
	}
	windows.ResumeThread(pi.Thread)
	windows.CloseHandle(pi.Thread)

	c := &console{t: t, hpc: hpc, proc: pi.Process, job: job, done: make(chan struct{}),
		in: os.NewFile(uintptr(inW), "conpty-in"), out: os.NewFile(uintptr(outR), "conpty-out")}
	go func() {
		b := make([]byte, 8192)
		for {
			n, err := c.out.Read(b)
			c.mu.Lock()
			c.buf = append(c.buf, b[:n]...)
			c.mu.Unlock()
			if err != nil {
				close(c.done)
				return
			}
		}
	}()
	return c
}

func envBlock(env []string) []uint16 {
	env = append([]string(nil), env...)
	sort.Slice(env, func(i, j int) bool { return strings.ToUpper(env[i]) < strings.ToUpper(env[j]) })
	var b []uint16
	for _, kv := range env {
		u, err := windows.UTF16FromString(kv)
		if err != nil {
			continue
		}
		b = append(b, u...) // includes the terminating 0
	}
	return append(b, 0)
}

var vtRe = regexp.MustCompile(`\x1b\[[0-?]*[ -/]*[@-~]|\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)|\x1b[()][0-9A-Za-z]|\x1b[=>78DEHMNOc]`)

// plain is the transcript without terminal control sequences.
func (c *console) plain() string {
	c.mu.Lock()
	s := string(c.buf)
	c.mu.Unlock()
	return strings.ReplaceAll(vtRe.ReplaceAllString(s, ""), "\r", "")
}

// expect waits until the transcript, past what was already matched, matches re.
func (c *console) expect(re string, d time.Duration) {
	c.t.Helper()
	rx := regexp.MustCompile(re)
	deadline := time.Now().Add(d)
	for {
		p := c.plain()
		if c.seen > len(p) {
			c.seen = len(p)
		}
		if loc := rx.FindStringIndex(p[c.seen:]); loc != nil {
			c.seen += loc[1]
			return
		}
		if time.Now().After(deadline) {
			c.kill()
			c.t.Fatalf("waited %v for %q; screen so far:\n%s", d, re, p)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// send types s into the console. "\r" is Enter, "\x03" is Ctrl+C.
func (c *console) send(s string) {
	time.Sleep(150 * time.Millisecond) // let the program reach its read
	c.in.Write([]byte(s))
}

// wait returns the exit code once the program ends.
func (c *console) wait(d time.Duration) uint32 {
	c.t.Helper()
	ev, _ := windows.WaitForSingleObject(c.proc, uint32(d/time.Millisecond))
	if ev != windows.WAIT_OBJECT_0 {
		c.kill()
		c.t.Fatalf("did not finish within %v; screen:\n%s", d, c.plain())
	}
	var code uint32
	windows.GetExitCodeProcess(c.proc, &code)
	c.close()
	return code
}

func (c *console) kill() {
	if c.job != 0 {
		windows.CloseHandle(c.job) // KILL_ON_JOB_CLOSE
		c.job = 0
	} else {
		windows.TerminateProcess(c.proc, 1)
	}
	c.close()
}

func (c *console) close() {
	if c.hpc != 0 {
		h := c.hpc
		c.hpc = 0
		closed := make(chan struct{})
		go func() { windows.ClosePseudoConsole(h); close(closed) }()
		select {
		case <-closed:
		case <-time.After(5 * time.Second):
		}
	}
	select {
	case <-c.done:
	case <-time.After(3 * time.Second):
	}
	c.in.Close()
	if c.job != 0 {
		windows.CloseHandle(c.job)
		c.job = 0
	}
}

// console runs claude-acc in a fresh pseudo-console of the sandbox.
func (s *sandbox) console(dir string, fake bool, args ...string) *console {
	s.t.Helper()
	for i, a := range args {
		if a == "add" && i+1 < len(args) {
			markUsed(args[i+1])
		}
	}
	return startConsole(s.t, accBin, args, s.environ(fake), dir)
}

func deleteTarget(t string) { secret.DeleteTarget(t) }
