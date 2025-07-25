package file

import (
	"golang.org/x/sys/windows"

	"github.com/go-sw/winfs/w32api"
)

// progress implements copy progress routine callback handler.
type progress interface {
	// user-defined handler to handle file operationprogress callback, should
	// return the appropriate PROGRESS_* value.
	HandleRoutine(
		totalFileSize int64,
		totalBytesTransferred int64,
		streamSize int64,
		streamBytesTransferred int64,
		streamNumber uint32,
		callbackReason uint32,
		sourceFile windows.Handle,
		destinationFile windows.Handle,
	) uintptr
}

// callbackHandler holds interface that implements routine handler.
type callbackHandler struct {
	handler progress
}

// CopyOptions contains options for copying file.
//
// https://learn.microsoft.com/en-us/windows/win32/api/winbase/nf-winbase-copyfileexw#parameters
type CopyOptions struct {
	// return an error if target file already exists
	NoOverWrite bool
	// can be restarted in case the copy fails, the copy will be much slower
	Restartable bool
	// open source file with write access
	OpenSourceForWrite bool
	// allow copying to desintation without encrypt(EFS) support, results in
	// creating file without encryption
	AllowDecryption bool
	// copy the symbolic link itself pointing to the same target
	CopySymlink bool
	// copy without using buffered I/O
	NoBuffer bool
	// request using tranferring compressed data
	RequestCompress bool
	// copy security descriptor from the source file
	CopySecurity bool
}

func (o *CopyOptions) asFlags() uint32 {
	if o == nil {
		return 0
	}

	var flags uint32

	if o.NoOverWrite {
		flags |= w32api.COPY_FILE_FAIL_IF_EXISTS
	}
	if o.Restartable {
		flags |= w32api.COPY_FILE_RESTARTABLE
	}
	if o.OpenSourceForWrite {
		flags |= w32api.COPY_FILE_OPEN_SOURCE_FOR_WRITE
	}
	if o.AllowDecryption {
		flags |= w32api.COPY_FILE_ALLOW_DECRYPTED_DESTINATION
	}
	if o.CopySymlink {
		flags |= w32api.COPY_FILE_COPY_SYMLINK
	}
	if o.NoBuffer {
		flags |= w32api.COPY_FILE_NO_BUFFERING
	}
	if o.RequestCompress {
		flags |= w32api.COPY_FILE_REQUEST_COMPRESSED_TRAFFIC
	}

	return flags
}

// MoveOptions contains options for moving file or directory.
//
// https://learn.microsoft.com/en-us/windows/win32/api/winbase/nf-winbase-movefilewithprogressw#parameters
type MoveOptions struct {
	// overwrite if the target exists instead of returning an error, does
	// not work if directory is to be moved to an existing directory
	OverWrite bool
	// allow moving to a different volume by copying and deleting
	AllowCopy bool
	// move the file after the system is restarted, can be used to move
	// system files such as paging file
	AfterReboot bool
	// flush after move operation is done
	WriteThrough bool
	// fail if the source file cannot be tracked for link after move
	KeepTrack bool
	// preserve security after move
	PreserveSecurity bool
}

func (o *MoveOptions) asFlags() uint32 {
	if o == nil {
		return 0
	}

	var flags uint32

	if o.OverWrite {
		flags |= windows.MOVEFILE_REPLACE_EXISTING
	}
	if o.AllowCopy {
		flags |= windows.MOVEFILE_COPY_ALLOWED
	}
	if o.AfterReboot {
		flags |= windows.MOVEFILE_DELAY_UNTIL_REBOOT
	}
	if o.WriteThrough {
		flags |= windows.MOVEFILE_WRITE_THROUGH
	}
	if o.KeepTrack {
		flags |= windows.MOVEFILE_FAIL_IF_NOT_TRACKABLE
	}

	return flags
}
