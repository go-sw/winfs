//go:build windows

package ads

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"golang.org/x/sys/windows"

	"github.com/go-sw/winfs/w32api"
	"github.com/puzpuzpuz/xsync/v4"
)

const (
	adsRename = 0x100000 // rename ADS, should be used alone for OpenFileADS
)

var (
	ErrNoADS       = errors.New("no alternate data stream found")
	ErrUnsupported = errors.New("file system does not support stream")
)

// parseStreamDataName parses ":streamname:$streamtype" format into name of stream.
// Returns stream name only if $streamtype is $DATA, otherwise returns empty string.
func parseStreamDataName(data w32api.WIN32_FIND_STREAM_DATA) string {
	dataStr := windows.UTF16ToString(data.StreamName[:])

	fields := strings.Split(dataStr, ":")

	name, strmType := fields[1], fields[2]

	// not a data stream type
	if strmType != "$DATA" {
		return ""
	}

	return name
}

// OpenFileADS opens data stream of the name from the given file with specified flag(used in os.OpenFile()),
// should be closed with (*os.File).Close() after use.
func OpenFileADS(path string, name string, openFlag int) (*os.File, error) {
	path = path + ":" + name

	u16Path, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}

	var access, mode, createmode uint32

	switch openFlag & (os.O_RDONLY | os.O_WRONLY | os.O_RDWR | adsRename) {
	case os.O_RDONLY:
		access = windows.FILE_READ_DATA | windows.SYNCHRONIZE
		mode = windows.FILE_SHARE_READ
	case os.O_WRONLY:
		access = windows.FILE_WRITE_DATA | windows.SYNCHRONIZE
		mode = windows.FILE_SHARE_WRITE
	case os.O_RDWR:
		access = windows.FILE_READ_DATA | windows.FILE_WRITE_DATA | windows.SYNCHRONIZE
		mode = windows.FILE_SHARE_READ | windows.FILE_SHARE_WRITE
	case adsRename:
		access = windows.DELETE | windows.SYNCHRONIZE
		mode = windows.FILE_SHARE_DELETE
	}

	switch openFlag & (os.O_CREATE | os.O_TRUNC | os.O_EXCL) {
	case os.O_CREATE | os.O_EXCL:
		createmode = windows.CREATE_NEW
	case os.O_CREATE | os.O_TRUNC:
		createmode = windows.CREATE_ALWAYS
	case os.O_CREATE:
		createmode = windows.OPEN_ALWAYS
	case os.O_TRUNC:
		createmode = windows.TRUNCATE_EXISTING
	default:
		createmode = windows.OPEN_EXISTING
	}

	if openFlag&os.O_APPEND != 0 {
		access &^= windows.FILE_WRITE_DATA
		access |= windows.FILE_APPEND_DATA
	}

	hnd, err := windows.CreateFile(
		u16Path,
		access,
		mode,
		nil,
		createmode,
		windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT,
		0,
	)
	if err != nil {
		return nil, err
	}

	return os.NewFile(uintptr(hnd), path), nil
}

// FileADS handles alternate data streams of a file.
type FileADS struct {
	Path          string
	StreamInfoMap *xsync.Map[string, int64]
}

// GetFileADS returns ADS handler with a map of alternate data streams
// from the specified file.
func GetFileADS(path string) (*FileADS, error) {
	var err error
	var absPath string // normalized path

	if strings.HasPrefix(path, "\\??\\") {
		// has NT Namespace prefix
		absPath = path
	} else {
		absPath, err = filepath.Abs(path)
		if err != nil {
			return nil, err
		}
	}

	ads := &FileADS{
		Path:          absPath,
		StreamInfoMap: xsync.NewMap[string, int64](),
	}

	if err = ads.CollectADS(); err != nil {
		return nil, err
	}

	return ads, err
}

// CollectADS collects name and size of alternate data streams of the file.
func (a *FileADS) CollectADS() error {
	findStrm, data, err := w32api.FindFirstStream(a.Path, w32api.FindStreamInfoStandard, 0)
	if err == windows.ERROR_HANDLE_EOF {
		// possible for directories or reparse points, files have at least one for unnamed data stream
		return ErrNoADS
	} else if err == windows.ERROR_INVALID_PARAMETER {
		return ErrUnsupported
	} else if err != nil {
		return err
	}

	a.StreamInfoMap.Clear()

	if strmName := parseStreamDataName(data); strmName != "" {
		a.StreamInfoMap.Store(strmName, data.StreamSize)
	}

	for {
		data, findErr := w32api.FindNextStream(findStrm)
		if findErr == windows.ERROR_HANDLE_EOF {
			// no more stream
			break
		} else if findErr != nil {
			err = findErr
			goto EXIT
		}

		if strmName := parseStreamDataName(data); strmName != "" {
			a.StreamInfoMap.Store(strmName, data.StreamSize)
		}
	}

EXIT:
	closeErr := w32api.FindClose(findStrm)
	if closeErr != nil {
		return fmt.Errorf("error: %w, FindClose err: %w", err, closeErr)
	}

	if a.StreamInfoMap.Size() == 0 {
		// return ErrNoADS for files
		return ErrNoADS
	}

	return err
}

// RenameADS renames alternate data stream with oldName to newName.
// If stream with newName exists, it will be overwitten if overwrite is true,
// otherwise return an error.
func (a *FileADS) RenameADS(oldName, newName string, overwrite bool) error {
	size, ok := a.StreamInfoMap.Load(oldName)
	if !ok {
		return fmt.Errorf("ADS \"%s\" does not exist", oldName)
	}

	f, err := OpenFileADS(a.Path, oldName, adsRename)
	if err != nil {
		return err
	}
	defer f.Close()

	// https://learn.microsoft.com/en-us/windows/win32/api/winbase/ns-winbase-file_rename_info#:~:text=The%20new%20name%20of%20an%20NTFS%20file%20stream%2C%20starting%20with%20%3A
	renameInfo, err := w32api.NewFileRenameInfo(":"+newName, overwrite)
	if err != nil {
		return err
	}

	//TODO fix 32bit ERROR_INVALID_NAME error
	if err = windows.SetFileInformationByHandle(
		windows.Handle(f.Fd()),
		windows.FileRenameInfo,
		&renameInfo[0],
		uint32(len(renameInfo)),
	); err != nil {
		return err
	}
	runtime.KeepAlive(f)

	a.StreamInfoMap.Delete(oldName)
	a.StreamInfoMap.Store(newName, size)

	return nil
}

// RemoveADS removes alternate data stream with the name.
func (a *FileADS) RemoveADS(name string) error {
	_, ok := a.StreamInfoMap.Load(name)
	if !ok {
		return fmt.Errorf("stream \"%s\" does not exist", name)
	}

	if err := os.Remove(a.Path + ":" + name); err != nil {
		return err
	}

	a.StreamInfoMap.Delete(name)

	return nil
}

// RemoveAllADS removes all alternate data streams from the file, leaving
// only the unnamed data stream, in which data are normally stored.
func (a *FileADS) RemoveAllADS() error {
	var err error

	// collect current ADS
	err = a.CollectADS()
	if err != nil {
		return err
	}

	a.StreamInfoMap.Range(func(name string, _ int64) bool {
		if removeErr := os.Remove(a.Path + ":" + name); removeErr != nil {
			err = errors.Join(err, removeErr)
			return true
		}
		a.StreamInfoMap.Delete(name)
		return true
	})

	return err
}
