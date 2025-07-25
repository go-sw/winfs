//go:build windows

package w32api

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// nt kernel functions

//sys	ntOpenFile(fileHandle *windows.Handle, accessMask uint32, objectAttributes *windows.OBJECT_ATTRIBUTES, ioStatusBlock *windows.IO_STATUS_BLOCK, sharedAccess uint32, openOptions uint32) (ntstatus error) = ntdll.NtOpenFile
//sys	ntClose(fileHandle windows.Handle) (ntstatus error) = ntdll.NtClose
//sys	ntQueryInformationFile(fileHandle windows.Handle, ioStatusBlock *windows.IO_STATUS_BLOCK, fileInformation unsafe.Pointer, length uint32, fileInformationClass int32) (ntstatus error) = ntdll.NtQueryInformationFile
//sys	NtSetSecurityObject(handle windows.Handle, securityInformation windows.SECURITY_INFORMATION, securityDescriptor *windows.SECURITY_DESCRIPTOR) (ntstatus error) = ntdll.NtSetSecurityObject

func NtOpenFile(accessMask uint32, objectAttributes *windows.OBJECT_ATTRIBUTES, ioStatusBlock *windows.IO_STATUS_BLOCK, sharedAccess uint32, openOptions uint32) (fileHandle windows.Handle, err error) {
	err = ntOpenFile(&fileHandle, accessMask, objectAttributes, ioStatusBlock, sharedAccess, openOptions)
	return
}

func NtClose(fileHandle windows.Handle) (err error) {
	err = ntClose(fileHandle)
	return
}

func NtQueryInformationFile(fileHandle windows.Handle, ioStatusBlock *windows.IO_STATUS_BLOCK, fileInformation unsafe.Pointer, length uint32, fileInformationClass int32) (err error) {
	err = ntQueryInformationFile(fileHandle, ioStatusBlock, fileInformation, length, fileInformationClass)
	return
}

// file operation functions

//sys	copyFileEx(existingFileName *uint16, newFileName *uint16, progressRoutine uintptr, data unsafe.Pointer, cancel *int32, copyFlags uint32) (err error) = kernel32.CopyFileExW
//sys	moveFileWithProgress(existingFileName *uint16, newFileName *uint16, progressRoutine uintptr, data unsafe.Pointer, flags uint32) (err error) = kernel32.MoveFileWithProgressW
//sys	setFileShortName(file windows.Handle, shortName *uint16) (err error) = kernel32.SetFileShortNameW
//sys	getShortPathName(longPath *uint16, shortPath *uint16, cchBuffer uint32) (length uint32, err error) = kernel32.GetShortPathNameW
//sys	getLongPathName(shortPath *uint16, longPath *uint16, cchBuffer uint32) (length uint32, err error) = kernel32.GetLongPathNameW

func CopyFileEx(existingFileName, newFileName string, progressRoutine uintptr, data unsafe.Pointer, cancel *int32, flags uint32) error {
	u16Exist, err := windows.UTF16PtrFromString(existingFileName)
	if err != nil {
		return err
	}

	u16New, err := windows.UTF16PtrFromString(newFileName)
	if err != nil {
		return err
	}

	return copyFileEx(u16Exist, u16New, progressRoutine, data, cancel, flags)
}

func MoveFileWithProgress(existingFileName, newFileName string, progressRoutine uintptr, data unsafe.Pointer, flags uint32) error {
	u16Old, err := windows.UTF16PtrFromString(existingFileName)
	if err != nil {
		return err
	}

	// use nil pointer for empty name for deleting with reboot
	var u16New *uint16
	if len(newFileName) != 0 {
		u16New, err = windows.UTF16PtrFromString(newFileName)
		if err != nil {
			return err
		}
	}

	return moveFileWithProgress(u16Old, u16New, progressRoutine, data, flags)
}

func SetFileShortName(file windows.Handle, shortName string) error {
	u16Short, err := windows.UTF16PtrFromString(shortName)
	if err != nil {
		return err
	}

	return setFileShortName(file, u16Short)
}

func GetShortPathName(path string) (string, error) {
	u16Path, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return "", err
	}

	ret, err := getShortPathName(u16Path, nil, 0)
	if ret == 0 {
		return "", err
	}

	buf := make([]uint16, ret)

	ret, err = getShortPathName(u16Path, &buf[0], uint32(len(buf)))
	if ret == 0 {
		return "", err
	}

	return windows.UTF16ToString(buf), nil
}

func GetLongPathName(shortName string) (string, error) {
	u16Short, err := windows.UTF16PtrFromString(shortName)
	if err != nil {
		return "", err
	}

	ret, err := getLongPathName(u16Short, nil, 0)
	if ret == 0 {
		return "", err
	}

	buf := make([]uint16, ret)

	ret, err = getLongPathName(u16Short, &buf[0], uint32(len(buf)))
	if ret == 0 {
		return "", err
	}

	return windows.UTF16ToString(buf), nil
}

// ADS(alternate data stream) functions

//sys	findFirstStream(fileName *uint16, infoLevel int32, findStreamData unsafe.Pointer, flags uint32) (hnd windows.Handle, err error) [failretval==windows.InvalidHandle] = kernel32.FindFirstStreamW
//sys	findNextStream(findStream windows.Handle, findStreamData unsafe.Pointer) (err error) = kernel32.FindNextStreamW
//sys	findClose(findFile windows.Handle) (err error) = kernel32.FindClose

func FindFirstStream(fileName string, infoLevel int32, flags uint32) (hnd windows.Handle, data WIN32_FIND_STREAM_DATA, err error) {
	wStr, err := windows.UTF16PtrFromString(fileName)
	if err != nil {
		return
	}

	// returned error
	// windows.ERROR_HANDLE_EOF if there is no stream
	// windows.ERROR_INVALID_PARAMETER for unsupported file system
	hnd, err = findFirstStream(wStr, infoLevel, unsafe.Pointer(&data), flags) // flags should be 0
	return
}

func FindNextStream(findStream windows.Handle) (data WIN32_FIND_STREAM_DATA, err error) {
	err = findNextStream(findStream, unsafe.Pointer(&data))

	// returns windows.ERROR_HANDLE_EOF if there is no more stream
	return
}

func FindClose(findFile windows.Handle) (err error) {
	err = findClose(findFile)
	return
}

// backup functions

//sys	backupRead(file windows.Handle, buffer *byte, numberOfBytesToRead uint32, numberOfBytesRead *uint32, abort bool, processSecurity bool, context *uintptr) (err error) = kernel32.BackupRead
//sys	backupSeek(file windows.Handle, lowBytesToSeek uint32, highBytesToSeek uint32, lowBytesSeeked *uint32, highBytesSeeked *uint32, context *uintptr) (err error) = kernel32.BackupSeek
//sys	backupWrite(file windows.Handle, buffer *byte, numberOfBytesToRead uint32, numberOfBytesRead *uint32, abort bool, processSecurity bool, context *uintptr) (err error) = kernel32.BackupWrite

// BackupRead reads data associated with a specified file or directory.
//
// https://learn.microsoft.com/en-us/windows/win32/api/winbase/nf-winbase-backupread
func BackupRead(file windows.Handle, buffer []byte, abort, processSecurity bool, context *uintptr) (bytesRead uint32, err error) {
	var b *byte
	if len(buffer) > 0 {
		b = &buffer[0]
	}

	err = backupRead(file, b, uint32(len(buffer)), &bytesRead, abort, processSecurity, context)
	return
}

/*
BackupSeek skips portion of a data stream.

https://learn.microsoft.com/en-us/windows/win32/api/winbase/nf-winbase-backupseek

caveats:

If the current stream offset is in the middle of the data, it succeeds if
and only if the remaining data is not smaller than the seeked size, setting
the seeked size to the given size.

However, if the current stream offset is in the middle of backup header, it fails
without setting the last error value(eventually returns ERROR_SUCCESS(0x0))
and does nothing(offset does not increase).

If the given seek size is larger than the currently remaining data size, it fails
and sets the last error to ERROR_SEEK(0x19), but the offset actually advances to
the starting offset of next stream header while keeping the seeked size to 0.
*/
func BackupSeek(file windows.Handle, offset uint64, context *uintptr) (seeked uint64, err error) {
	var seekedLow, seekedHigh uint32
	err = backupSeek(file, uint32(offset), uint32(offset>>32), &seekedLow, &seekedHigh, context)
	return (uint64(seekedHigh) << 32) | uint64(seekedLow), err
}

// BackupWrite writes the associated data to specified file or directory.
//
// https://learn.microsoft.com/en-us/windows/win32/api/winbase/nf-winbase-backupwrite
func BackupWrite(file windows.Handle, buffer []byte, abort, processSecurity bool, context *uintptr) (bytesWritten uint32, err error) {
	var b *byte
	if len(buffer) > 0 {
		b = &buffer[0]
	}

	err = backupWrite(file, b, uint32(len(buffer)), &bytesWritten, abort, processSecurity, context)
	return
}

// EA(extended attributes) functions

//sys	ntSetEaFile(fileHandle windows.Handle, ioStatusBlock *windows.IO_STATUS_BLOCK, buffer unsafe.Pointer, length uint32) (ntstatus error) = ntdll.NtSetEaFile
//sys	ntQueryEaFile(fileHandle windows.Handle, ioStatusBlock *windows.IO_STATUS_BLOCK, buffer unsafe.Pointer, length uint32, returnSingleEntry bool, eaList unsafe.Pointer, eaListLength uint32, eaIndex *uint32, restartScan bool) (ntstatus error) = ntdll.NtQueryEaFile

func NtSetEaFile(fileHandle windows.Handle, ioStatusBlock *windows.IO_STATUS_BLOCK, buffer unsafe.Pointer, length uint32) (err error) {
	err = ntSetEaFile(fileHandle, ioStatusBlock, buffer, length)
	return
}

func NtQueryEaFile(fileHandle windows.Handle, ioStatusBlock *windows.IO_STATUS_BLOCK, buffer unsafe.Pointer, length uint32, returnSingleEntry bool, eaList unsafe.Pointer, eaListLength uint32, eaIndex *uint32, restartScan bool) (err error) {
	err = ntQueryEaFile(fileHandle, ioStatusBlock, buffer, length, returnSingleEntry, eaList, eaListLength, eaIndex, restartScan)
	return
}

// EFS(encrypted file system) functions

//sys	fileEncryptionStatus(lpFileName *uint16, lpStatus *uint32) (err error) = advapi32.FileEncryptionStatusW
//sys	encryptionDisable(dirPath *uint16, disable bool) (err error) = advapi32.EncryptionDisable
//sys	encryptFile(lpFileName *uint16) (err error) = advapi32.EncryptFileW
//sys	decryptFile(lpFileName *uint16, dwReserved uint32) (err error) = advapi32.DecryptFileW

func FileEncryptionStatus(fileName string) (status uint32, err error) {
	u16ptr, err := windows.UTF16PtrFromString(fileName)
	if err != nil {
		return
	}

	err = fileEncryptionStatus(u16ptr, &status)

	return
}

func EncryptionDisable(dirPath string, disable bool) error {
	u16ptr, err := windows.UTF16PtrFromString(dirPath)
	if err != nil {
		return err
	}

	err = encryptionDisable(u16ptr, disable)

	return err
}

func EncryptFile(fileName string) error {
	u16ptr, err := windows.UTF16PtrFromString(fileName)
	if err != nil {
		return err
	}

	return encryptFile(u16ptr)
}

// reserved should be 0
func DecryptFile(fileName string, reserved uint32) error {
	u16ptr, err := windows.UTF16PtrFromString(fileName)
	if err != nil {
		return err
	}

	return decryptFile(u16ptr, reserved)
}

// EFS files and user key functions

//sys	addUsersToEncryptedFile(lpFileName *uint16, pEncryptionCertificates *ENCRYPTION_CERTIFICATE_LIST) (ret error) = advapi32.AddUsersToEncryptedFileW
//sys	duplicateEncryptionInfoFile(srcFileName *uint16, dstFileName *uint16, creationDistribution uint32, attributes uint32, lpSecurityAttributes *windows.SecurityAttributes) (ret error) = advapi32.DuplicateEncryptionInfoFile
//sys	FreeEncryptionCertificateHashList(pUsers *ENCRYPTION_CERTIFICATE_HASH_LIST) = advapi32.FreeEncryptionCertificateHashList
//sys	queryRecoveryAgentsOnEncryptedFile(lpFileName *uint16, pRecoveryAgents *ENCRYPTION_CERTIFICATE_HASH_LIST) (ret error) = advapi32.QueryRecoveryAgentsOnEncryptedFile
//sys	queryUsersOnEncryptedFile(lpFileName *uint16, pUsers *ENCRYPTION_CERTIFICATE_HASH_LIST) (ret error) = advapi32.QueryUsersOnEncryptedFile
//sys	removeUsersFromEncryptedFile(lpFileName *uint16, pHashes *ENCRYPTION_CERTIFICATE_HASH_LIST) (ret error) = advapi32.RemoveUsersFromEncryptedFile
//sys	SetUserFileEncryptionKey(pEncryptionCertificate *ENCRYPTION_CERTIFICATE) (ret error) = advapi32.SetUserFileEncryptionKey

func AddUsersToEncryptedFile(fileName string, encryptionCertificates []ENCRYPTION_CERTIFICATE) error {
	u16ptr, err := windows.UTF16PtrFromString(fileName)
	if err != nil {
		return err
	}

	var list ENCRYPTION_CERTIFICATE_LIST
	list.NumUser = uint32(len(encryptionCertificates))
	list.Users = (*ENCRYPTION_CERTIFICATE)(unsafe.Pointer(&encryptionCertificates[0]))

	return addUsersToEncryptedFile(u16ptr, &list)
}

func DuplicateEncryptionInfoFile(srcFileName, dstFilename string, creationDistribution, attributes uint32, securityAttributes *windows.SecurityAttributes) error {
	u16src, err := windows.UTF16PtrFromString(srcFileName)
	if err != nil {
		return err
	}
	u16dst, err := windows.UTF16PtrFromString(dstFilename)
	if err != nil {
		return err
	}

	return duplicateEncryptionInfoFile(u16src, u16dst, creationDistribution, attributes, securityAttributes)
}

func QueryRecoveryAgentsOnEncryptedFile(fileName string) (ENCRYPTION_CERTIFICATE_HASH_LIST, error) {
	var list ENCRYPTION_CERTIFICATE_HASH_LIST

	u16str, err := windows.UTF16PtrFromString(fileName)
	if err != nil {
		return list, err
	}

	err = queryRecoveryAgentsOnEncryptedFile(u16str, &list)
	if err != nil {
		return list, err
	}

	return list, nil
}

func QueryUsersOnEncryptedFile(fileName string) (ENCRYPTION_CERTIFICATE_HASH_LIST, error) {
	var list ENCRYPTION_CERTIFICATE_HASH_LIST

	u16str, err := windows.UTF16PtrFromString(fileName)
	if err != nil {
		return list, err
	}

	err = queryUsersOnEncryptedFile(u16str, &list)
	if err != nil {
		return list, err
	}

	return list, nil
}

func RemoveUsersFromEncryptedFile(fileName string, hashes []ENCRYPTION_CERTIFICATE_HASH) error {
	u16str, err := windows.UTF16PtrFromString(fileName)
	if err != nil {
		return err
	}

	var list ENCRYPTION_CERTIFICATE_HASH_LIST
	list.NumCertHash = uint32(len(hashes))
	list.Users = (*ENCRYPTION_CERTIFICATE_HASH)(unsafe.Pointer(&hashes[0]))

	return removeUsersFromEncryptedFile(u16str, &list)
}

// EFS file backup and restore functions

//sys	openEncryptedFileRaw(lpFileName *uint16, ulFlags uint32, pvContext *unsafe.Pointer) (ret error) = advapi32.OpenEncryptedFileRawW
//sys	CloseEncryptedFileRaw(pvContext unsafe.Pointer) = advapi32.CloseEncryptedFileRaw
//sys	ReadEncryptedFileRaw(pfExportCallback uintptr, pvCallbackContext unsafe.Pointer, pvContext unsafe.Pointer) (ret error) = advapi32.ReadEncryptedFileRaw
//sys	WriteEncryptedFileRaw(pfImportCallback uintptr, pvCallbackContext unsafe.Pointer, pvContext unsafe.Pointer) (ret error) = advapi32.WriteEncryptedFileRaw

func OpenEncryptedFileRaw(fileName string, flags uint32) (context unsafe.Pointer, err error) {
	u16str, err := windows.UTF16PtrFromString(fileName)
	if err != nil {
		return
	}

	err = openEncryptedFileRaw(u16str, flags, &context)

	return
}
