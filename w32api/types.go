package w32api

import (
	"io"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	anySize = 1
)

// alternate data stream

// FindFirstStreamW InfoLevel constant
const (
	FindStreamInfoStandard = 0
)

type WIN32_FIND_STREAM_DATA struct {
	StreamSize int64
	// ":streamname:$streamtype", possible $streamtype: $DATA, $INDEX_ALLOCATION, $BITMAP
	StreamName [windows.MAX_PATH + 36]uint16
}

// backup

type WIN32_STREAM_ID struct {
	StreamId         uint32
	StreamAttributes uint32
	Size             int64
	StreamNameSize   uint32
	// StreamName    [StreamNameSize]uint16
}

type FILE_LINKS_INFORMATION struct {
	BytesNeeded     uint32
	EntriesReturned uint32
	// [EntriesReturned]FILE_LINK_ENTRY_INFORMATION
	Entry FILE_LINK_ENTRY_INFORMATION
}

type FILE_LINK_ENTRY_INFORMATION struct {
	NextEntryOffset uint32
	ParentFileId    int64
	Filenamelength  uint32
	FileName        [anySize]uint16
}

// EA(extended attributes)

const (
	FILE_NEED_EA = 0x80 // file should be interpreted with Extended Attributes(EA)
)

// file information class type used in NtQueryInformationFile and NtSetInformationFile.
const (
	FileEaInformation       = 7
	FileHardLinkInformation = 46
)

type FILE_EA_INFORMATION struct {
	EaSize uint32
}

// 4 bytes aligned
type FILE_FULL_EA_INFORMATION struct {
	NextEntryOffset uint32
	Flags           uint8
	EaNameLength    uint8
	EaValueLength   uint16
	// 1 byte for ASCII character, 2 or more bytes for non-ASCII character, looks like the supported character follows the active codepage of the computer.., for English users, it might be cp1252, cp850...
	//
	// As like file names in NTFS, the name of EA is case-insensitive and shown using capital letters when queried.
	EaName [anySize]int8 // EaNameLength[int8]
	//_ [1]byte // '\0'

	/* EaValue [EaValueLength]byte */
}

// 4 bytes aligned
type FILE_GET_EA_INFORMATION struct {
	NextEntryOffset uint32
	EaNameLength    uint8
	EaName          []int8 // [EaNameLength]int8
	//_ [1]byte // null terminator
}

// EFS(encrypted file system) types

// flags for OpenEncryptedFileRaw
const (
	CREATE_FOR_IMPORT = 1
	CREATE_FOR_DIR    = 2
	OVERWRITE_HIDDEN  = 4
)

type EFS_CERTIFICATE_BLOB struct {
	CertEncodingType uint32
	CbData           uint32
	PbData           *byte
}

type ENCRYPTION_CERTIFICATE struct {
	TotalLength uint32
	UserSid     *windows.SID
	CertBlob    *EFS_CERTIFICATE_BLOB
}

type ENCRYPTION_CERTIFICATE_LIST struct {
	NumUser uint32
	Users   *ENCRYPTION_CERTIFICATE // []ENCRYPTION_CERTIFICATE
}

type EFS_HASH_BLOB struct {
	CbData uint32
	PbData *byte
}

type ENCRYPTION_CERTIFICATE_HASH struct {
	TotalLength        uint32
	UserSid            *windows.SID
	Hash               *EFS_HASH_BLOB
	DisplayInformation *uint16
}

type ENCRYPTION_CERTIFICATE_HASH_LIST struct {
	NumCertHash uint32
	Users       *ENCRYPTION_CERTIFICATE_HASH // []ENCRYPTION_CERTIFICATE_HASH
}

// CopyProgressRoutine
type LPPROGRESS_ROUTINE func(
	totalFileSize int64,
	totalBytesTransferred int64,
	streamSize int64,
	streamBytesTransferred int64,
	streamNumber uint32,
	callbackReason uint32,
	sourceFile windows.Handle,
	destinationFile windows.Handle,
	data unsafe.Pointer,
) /* uint32 */ uintptr

// CopyProgressRoutine callback reason
const (
	CALLBACK_CHUNK_FINISHED = 0x00000000
	CALLBACK_STREAM_FINISH  = 0x00000001
)

// CopyProgressRoutine return value
const (
	PROGRESS_CONTINUE = 0
	PROGRESS_CANCEL   = 1
	PROGRESS_STOP     = 2
	PROGRESS_QUIET    = 3
)

// CopyFileEx flags
const (
	COPY_FILE_FAIL_IF_EXISTS              = 0x00000001
	COPY_FILE_RESTARTABLE                 = 0x00000002
	COPY_FILE_OPEN_SOURCE_FOR_WRITE       = 0x00000004
	COPY_FILE_ALLOW_DECRYPTED_DESTINATION = 0x00000008
	COPY_FILE_COPY_SYMLINK                = 0x00000800
	COPY_FILE_NO_BUFFERING                = 0x00001000
	COPY_FILE_REQUEST_COMPRESSED_TRAFFIC  = 0x10000000
)

// interface that implements Write, Seek and Close
type WriteSeekCloser interface {
	io.Writer
	io.Seeker
	io.Closer
}
