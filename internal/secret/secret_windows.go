//go:build windows

package secret

import (
	"errors"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Windows Credential Manager, through advapi32 directly: generic credentials
// named "claude-code-token:<account>", encrypted by Windows for this user.

var (
	advapi32       = windows.NewLazySystemDLL("advapi32.dll")
	procCredRead   = advapi32.NewProc("CredReadW")
	procCredWrite  = advapi32.NewProc("CredWriteW")
	procCredDelete = advapi32.NewProc("CredDeleteW")
	procCredFree   = advapi32.NewProc("CredFree")
)

const (
	credTypeGeneric         = 1
	credPersistLocalMachine = 2
)

// credentialW mirrors the Win32 CREDENTIALW struct.
type credentialW struct {
	Flags              uint32
	Type               uint32
	TargetName         *uint16
	Comment            *uint16
	LastWritten        windows.Filetime
	CredentialBlobSize uint32
	CredentialBlob     *byte
	Persist            uint32
	AttributeCount     uint32
	Attributes         uintptr
	TargetAlias        *uint16
	UserName           *uint16
}

func target(name string) *uint16 {
	p, _ := windows.UTF16PtrFromString(service() + ":" + name)
	return p
}

func get(name string) (string, error) { return readTarget(target(name)) }

// ReadTarget reads the generic credential named t — used for Claude Code's
// own entry, which Bun.secrets writes as "<service>/<name>".
func ReadTarget(t string) (string, bool) {
	p, err := windows.UTF16PtrFromString(t)
	if err != nil {
		return "", false
	}
	v, err := readTarget(p)
	return v, err == nil
}

func readTarget(t *uint16) (string, error) {
	var pcred *credentialW
	r, _, err := procCredRead.Call(uintptr(unsafe.Pointer(t)), credTypeGeneric, 0,
		uintptr(unsafe.Pointer(&pcred)))
	if r == 0 {
		if errors.Is(err, windows.ERROR_NOT_FOUND) {
			return "", ErrNotFound
		}
		return "", err
	}
	defer procCredFree.Call(uintptr(unsafe.Pointer(pcred)))
	if pcred.CredentialBlobSize == 0 {
		return "", nil
	}
	blob := unsafe.Slice(pcred.CredentialBlob, pcred.CredentialBlobSize)
	return string(blob), nil
}

func set(name, token string) error {
	blob := []byte(token)
	comment, _ := windows.UTF16PtrFromString("Claude Code account token (managed by claude-acc)")
	user, _ := windows.UTF16PtrFromString(name)
	cred := credentialW{
		Type:               credTypeGeneric,
		TargetName:         target(name),
		Comment:            comment,
		CredentialBlobSize: uint32(len(blob)),
		Persist:            credPersistLocalMachine,
		UserName:           user,
	}
	if len(blob) > 0 {
		cred.CredentialBlob = &blob[0]
	}
	r, _, err := procCredWrite.Call(uintptr(unsafe.Pointer(&cred)), 0)
	if r == 0 {
		return err
	}
	return nil
}

func del(name string) error {
	r, _, err := procCredDelete.Call(uintptr(unsafe.Pointer(target(name))), credTypeGeneric, 0)
	if r == 0 {
		if errors.Is(err, windows.ERROR_NOT_FOUND) {
			return ErrNotFound
		}
		return err
	}
	return nil
}
