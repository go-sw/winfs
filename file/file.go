//go:build windows

package file

import (
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/go-sw/winfs/w32api"
)

var (
	// global progress callback for file operation
	progressCb uintptr
)

// initCb initializes a global callback to reduce callback limit pressure,
// this will only be called if callback is defined
func initCb() {
	if progressCb != 0 {
		return
	}

	// "runtime/syscall_windows.go".compileCallback has mutex by itself
	// and does the deduplication
	progressCb = w32api.NewCopyProgressRoutine(func(
		totalFileSize,
		totalBytesTransferred,
		streamSize,
		streamBytesTransferred int64,
		streamNumber,
		callbackReason uint32,
		sourceFile,
		destinationFile windows.Handle,
		data unsafe.Pointer,
	) uintptr {
		if data == nil {
			return w32api.PROGRESS_CONTINUE
		}
		handler := (*callbackHandler)(data).handler
		if handler == nil {
			return w32api.PROGRESS_CONTINUE
		}

		return handler.HandleRoutine(
			totalFileSize,
			totalBytesTransferred,
			streamSize,
			streamBytesTransferred,
			streamNumber,
			callbackReason,
			sourceFile,
			destinationFile,
		)
	})
}

// fixPath changes path to absolute path with long name support
func fixPath(path string) string {
	var absPath string

	if len(path) >= 4 && (path[:4] == `\\?\` || path[:4] == `\??\`) {
		// path with prefix should be a absolute path
		absPath = path
	} else {
		var err error
		absPath, err = filepath.Abs(path)
		if err == nil {
			// prepend long path support prefix
			absPath = `\\?\` + absPath
		}
	}

	return absPath
}

func setSecurityInfo(target string, sd *windows.SECURITY_DESCRIPTOR) error {
	u16target, err := windows.UTF16PtrFromString(fixPath(target))
	if err != nil {
		return err
	}

	setSacl := true

	hnd, err := windows.CreateFile(
		u16target,
		windows.WRITE_DAC|windows.WRITE_OWNER|windows.ACCESS_SYSTEM_SECURITY,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT,
		0,
	)
	if err != nil {
		// retry without sacl
		hnd, err = windows.CreateFile(
			u16target,
			windows.WRITE_DAC|windows.WRITE_OWNER,
			windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
			nil,
			windows.OPEN_EXISTING,
			windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT,
			0,
		)
		if err != nil {
			return &os.PathError{Op: "CreateFile", Path: target, Err: err}
		}
		setSacl = false
	}

	defer func() {
		if err != nil {
			windows.CloseHandle(hnd)
		}
	}()

	var secInfo windows.SECURITY_INFORMATION = windows.OWNER_SECURITY_INFORMATION | windows.GROUP_SECURITY_INFORMATION | windows.DACL_SECURITY_INFORMATION
	if setSacl {
		secInfo |= windows.SACL_SECURITY_INFORMATION
	}

	err = w32api.NtSetSecurityObject(
		hnd,
		secInfo,
		sd,
	)
	if err != nil {
		return &os.PathError{Op: "NtSetSecurityObject", Path: target, Err: err}
	}

	if closeErr := windows.CloseHandle(hnd); closeErr != nil {
		return &os.PathError{Op: "CloseHandle", Path: target, Err: closeErr}
	}

	return nil
}

// WinFile handles Windows specific file operation with progress
// callback and cancellation.
type WinFile struct {
	path     string // absoulte path with long path support
	callback *callbackHandler
	cbAddr   uintptr
}

// NewWinFile returns new [WinFile] to be used for file operation,
// the specified file should exist.
func NewWinFile(path string) (*WinFile, error) {
	var err error
	absPath := fixPath(path)

	// check if the specified file exists
	_, err = os.Lstat(absPath)
	if err != nil {
		return nil, err
	}

	return &WinFile{
		path: absPath,
	}, nil
}

// SetHandler sets handler which implements [progress]
// which has the following method
//
//	HandleRoutine(
//		totalFileSize int64,
//		totalBytesTransferred int64,
//		streamSize int64,
//		streamBytesTransferred int64,
//		streamNumber uint32,
//		callbackReason uint32,
//		sourceFile windows.Handle,
//		destinationFile windows.Handle,
//	) uintptr
//
// setting it to nil removes callback for this file.
func (f *WinFile) SetHandler(handler progress) {
	if handler == nil {
		// do not use callback for later operations
		f.cbAddr = 0
		f.callback = nil
		return
	}

	initCb()
	f.cbAddr = progressCb

	f.callback = &callbackHandler{
		handler: handler,
	}
}

// CopySecurity copies security descriptor to a target.
func (f *WinFile) CopySecurity(target string) error {
	sd, err := f.GetSecurityInfo()
	if err != nil {
		return err
	}

	return setSecurityInfo(target, sd)
}

// Copy copies the underlying file to the destination, cancel pointer can optionally be a
// pointer to int32 value which can be set to non-zero value to cancel copy operation.
func (f *WinFile) Copy(destination string, options *CopyOptions, cancel *int32) error {
	err := w32api.CopyFileEx(f.path, fixPath(destination), f.cbAddr, unsafe.Pointer(f.callback), cancel, options.asFlags())
	if err != nil {
		return &os.PathError{Op: "CopyFileEx", Path: f.path, Err: err}
	}

	if options.CopySecurity {
		if err = f.CopySecurity(destination); err != nil {
			return err
		}
	}

	return nil
}

// Move moves the underlying file or directory to the destination.
func (f *WinFile) Move(destination string, options *MoveOptions) error {
	var sd *windows.SECURITY_DESCRIPTOR
	var err error

	if options.PreserveSecurity {
		sd, err = f.GetSecurityInfo()
		if err != nil {
			return err
		}
	}

	err = w32api.MoveFileWithProgress(f.path, fixPath(destination), f.cbAddr, unsafe.Pointer(f.callback), options.asFlags())
	if err != nil {
		return &os.PathError{Op: "MoveFileWithProgress", Path: f.path, Err: err}
	}

	if options.PreserveSecurity {
		if err = setSecurityInfo(destination, sd); err != nil {
			return err
		}
	}

	return nil
}

func (f *WinFile) GetSecurityInfo() (*windows.SECURITY_DESCRIPTOR, error) {
	sd, err := windows.GetNamedSecurityInfo(
		f.path,
		windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|
			windows.GROUP_SECURITY_INFORMATION|
			windows.DACL_SECURITY_INFORMATION|
			windows.SACL_SECURITY_INFORMATION,
	)
	if err != nil {
		// retry without sacl
		sd, err = windows.GetNamedSecurityInfo(
			f.path,
			windows.SE_FILE_OBJECT,
			windows.OWNER_SECURITY_INFORMATION|
				windows.GROUP_SECURITY_INFORMATION|
				windows.DACL_SECURITY_INFORMATION,
		)
		if err != nil {
			return nil, err
		}
	}

	return sd, nil
}

// GetShortName returns short path name(8.3 filename) of this file.
func (f *WinFile) GetShortName() (string, error) {
	return w32api.GetShortPathName(f.path)
}

// SetShortName sets short path name(8.3 filename) of this file, the
// file needs to be in NTFS volume.
func (f *WinFile) SetShortName(shortName string) error {
	u16Path, err := windows.UTF16PtrFromString(f.path)
	if err != nil {
		return err
	}

	var hnd windows.Handle
	defer func() {
		if err != nil {
			windows.CloseHandle(hnd)
		}
	}()

	hnd, err = windows.CreateFile(
		u16Path,
		windows.GENERIC_WRITE|windows.DELETE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT,
		0,
	)
	if err != nil {
		return err
	}

	if err = w32api.SetFileShortName(hnd, shortName); err != nil {
		return err
	}

	return windows.CloseHandle(hnd)
}
