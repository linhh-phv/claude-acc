//go:build windows

package fsx

import (
	"os"
	"time"

	"golang.org/x/sys/windows"
)

// rename retries briefly: on Windows replacing a file fails while another
// process has it open without delete-sharing — Claude Code reads
// settings.json on every request.
func rename(from, to string) error {
	var err error
	for i := 0; i < 20; i++ {
		if err = os.Rename(from, to); err == nil {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return err
}

// protect gives the file an access list of its own — this user and SYSTEM,
// nothing inherited — before any token is written into it. Mode 0600 means
// nothing on Windows: a pin in a repo would otherwise inherit the folder's
// access list, and under C:\src that often lets every signed-in user read it.
func protect(path string) {
	u, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return
	}
	sd, err := windows.SecurityDescriptorFromString("D:P(A;;FA;;;" + u.User.Sid.String() + ")(A;;FA;;;SY)")
	if err != nil {
		return
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		return
	}
	windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, dacl, nil)
}
