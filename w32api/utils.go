//go:build windows

package w32api

import (
	"bytes"
	"errors"
	"unsafe"

	"golang.org/x/sys/windows"
)

/*
	typedef struct _FILE_RENAME_INFO {
	#if _WIN32_WINNT >= _WIN32_WINNT_WIN10_RS1
		__C89_NAMELESS union {
        	BOOLEAN ReplaceIfExists; // FileRenameInformation
        	ULONG Flags;             // FileRenameInformationEx
		};
	#else
		BOOLEAN ReplaceIfExists;
	#endif
		HANDLE RootDirectory;
		DWORD FileNameLength;
		WCHAR FileName[1];
	} FILE_RENAME_INFO,*PFILE_RENAME_INFO;

	typedef struct _FILE_LINK_INFORMATION {
	#if (_WIN32_WINNT >= _WIN32_WINNT_WIN10_RS5)
		union {
			BOOLEAN ReplaceIfExists;  // FileLinkInformation
			ULONG Flags;              // FileLinkInformationEx
		} DUMMYUNIONNAME;
	#else
		BOOLEAN ReplaceIfExists;
	#endif
		HANDLE RootDirectory;
		ULONG FileNameLength;
		WCHAR FileName[1];
	} FILE_LINK_INFORMATION, *PFILE_LINK_INFORMATION;
*/

// NewFileRenameInfo returns FILE_RENAME_INFO from winbase.h as bytes buffer
// to be used for renaming file.
func NewFileRenameInfo(newName string, replace bool) ([]byte, error) {
	return newLinkOrRenameBuffer(newName, replace)
}

// NewFileLinkInfo return FILE_LINK_INFORMATION from ntifs.h as bytes buffer
// to be used to create a hard link for a file.
func NewFileLinkInfo(linkName string, replace bool) ([]byte, error) {
	return newLinkOrRenameBuffer(linkName, replace)
}

func newLinkOrRenameBuffer(name string, replace bool) ([]byte, error) {
	if len(name) == 0 {
		return nil, errors.New("name is empty")
	}

	wordSize := unsafe.Sizeof(windows.Handle(0)) // word size for alignment to match C struct

	var renameInfo bytes.Buffer
	var replaceIfExists int32

	if replace {
		replaceIfExists = 1 // TRUE
	}

	// set ReplaceIfExists DWORD(uint32) to true
	renameInfo.Write(unsafe.Slice((*byte)(unsafe.Pointer(&replaceIfExists)), unsafe.Sizeof(replaceIfExists)))
	renameInfo.Write(make([]byte, wordSize-unsafe.Sizeof(replaceIfExists))) // alignment
	// RootDirectory HANDLE(NULL)
	renameInfo.Write(make([]byte, wordSize))

	u16Name, err := windows.UTF16FromString(name)
	if err != nil {
		return nil, err
	}

	fileNameLength := uint32((len(u16Name) - 1) * 2) // length in bytes without NULL terminaton

	renameInfo.Write(unsafe.Slice((*byte)(unsafe.Pointer(&fileNameLength)), unsafe.Sizeof(fileNameLength))) // FileNameLength DWORD(uint32)
	renameInfo.Write(unsafe.Slice((*byte)(unsafe.Pointer(&u16Name[0])), len(u16Name)*2))                    // FileName []uint16(WCHAR[])

	return renameInfo.Bytes(), nil
}

func NewCopyProgressRoutine(cb LPPROGRESS_ROUTINE) uintptr {
	return windows.NewCallback(cb)
}
