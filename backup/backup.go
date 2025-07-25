// Package backup implements [io.Reader] and [io.Writer] and [io.Seeker] for Microsoft NT Backup File.
package backup

import (
	"bytes"
	"errors"
	"io"
	"os"
	"runtime"
	"slices"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/go-sw/winfs/w32api"
)

// bkupStruct provides functions for [MS-BKUP]
//
// https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-bkup
type bkupStruct struct {
	h               windows.Handle
	ctx             uintptr
	mu              sync.Mutex
	path            string // file path
	restore         bool
	processSecurity bool
	cleanup         runtime.Cleanup
}

func newBkupStruct(path string, restore, processSecurity, overwrite bool) (*bkupStruct, error) {
	u16Path, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}

	access := uint32(windows.GENERIC_READ)
	mode := uint32(windows.FILE_SHARE_READ)
	createmode := uint32(windows.OPEN_EXISTING)
	attrs := uint32(windows.FILE_FLAG_BACKUP_SEMANTICS | windows.FILE_OPEN_REPARSE_POINT)

	if restore {
		access = windows.GENERIC_WRITE
		if overwrite {
			createmode = windows.CREATE_ALWAYS
		} else {
			createmode = windows.CREATE_NEW
		}
		mode = windows.FILE_SHARE_WRITE
		if processSecurity {
			access |= windows.WRITE_OWNER | windows.WRITE_DAC
		}
	}

	if processSecurity {
		access |= windows.ACCESS_SYSTEM_SECURITY
	}

	hnd, err := windows.CreateFile(
		u16Path,
		access,
		mode,
		nil, // set later with [w32api.BackupWrite] for restore
		createmode,
		attrs,
		0,
	)
	if err != nil {
		return nil, &os.PathError{Op: "CreateFile", Path: path, Err: err}
	}

	entry := &bkupStruct{
		h:               hnd,
		path:            path,
		restore:         restore,
		processSecurity: processSecurity,
	}

	return entry, nil
}

func (s *bkupStruct) close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var err error

	if s.ctx != 0 {
		if s.restore {
			if _, finErr := w32api.BackupWrite(0, nil, true, false, &s.ctx); finErr != nil {
				err = errors.Join(err, &os.PathError{Op: "BackupWrite", Path: s.path, Err: finErr})
			}
		} else {
			if _, finErr := w32api.BackupRead(0, nil, true, false, &s.ctx); finErr != nil {
				err = errors.Join(err, &os.PathError{Op: "BackupRead", Path: s.path, Err: finErr})
			}
		}
		if err == nil {
			s.ctx = 0
		}
	}
	if s.h != 0 {
		closeErr := windows.CloseHandle(s.h)
		if closeErr != nil {
			err = errors.Join(err, &os.PathError{Op: "CloseHandle", Path: s.path, Err: closeErr})
		} else {
			s.h = 0
		}
	}

	s.cleanup.Stop()
	return err
}

func (s *bkupStruct) read(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	n, err := w32api.BackupRead(s.h, p, false, s.processSecurity, &s.ctx)
	if err != nil {
		return int(n), &os.PathError{Op: "BackupRead", Path: s.path, Err: err}
	}

	if n == 0 {
		return 0, io.EOF
	}

	return int(n), nil
}

func (s *bkupStruct) write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	n, err := w32api.BackupWrite(s.h, p, false, s.processSecurity, &s.ctx)
	if err != nil {
		return int(n), &os.PathError{Op: "BackupWrite", Path: s.path, Err: err}
	}

	if n == 0 {
		return 0, io.EOF
	}

	return int(n), nil
}

func (s *bkupStruct) seek(offset int64, whence int) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// [BackupSeek] only supports [io.SeekCurrent]
	if whence != io.SeekCurrent {
		return 0, errors.New("unsupported whence")
	}

	// n == offset if seek succeeds, see the caveats part of [BackupSeek]
	n, err := w32api.BackupSeek(s.h, uint64(offset), &s.ctx)
	switch err {
	case nil:
		if n == 0 {
			return 0, io.EOF
		}
		return int64(n), nil
	case windows.ERROR_SUCCESS:
		// not actually seeked as it will skip the header
		return int64(n), ErrSkipHeader
	case windows.ERROR_SEEK:
		// while it returns 0, it actually seeks to the next stream header
		return int64(n), err // return directly for comparision
	default:
		return int64(n), &os.PathError{Op: "BackupSeek", Path: s.path, Err: err}
	}
}

type BackupFileReader struct {
	s *bkupStruct
}

func (r *BackupFileReader) Close() error {
	return r.s.close()
}

func (r *BackupFileReader) Read(p []byte) (int, error) {
	n, err := r.s.read(p)
	runtime.KeepAlive(r)
	return n, err
}

func (r *BackupFileReader) Seek(offset int64, whence int) (int64, error) {
	ret, err := r.s.seek(offset, whence)
	runtime.KeepAlive(r)
	return ret, err
}

func NewBackupFileReader(path string, processSecurity bool) (*BackupFileReader, error) {
	st, err := newBkupStruct(path, false, processSecurity, false)
	if err != nil {
		return nil, err
	}

	r := &BackupFileReader{st}
	r.s.cleanup = runtime.AddCleanup(r, func(s *bkupStruct) { s.close() }, st)

	return r, nil
}

type BackupFileWriter struct {
	s *bkupStruct
}

func (w *BackupFileWriter) Close() error {
	return w.s.close()
}

func (w *BackupFileWriter) Write(p []byte) (int, error) {
	n, err := w.s.write(p)
	runtime.KeepAlive(w)
	return n, err
}

func (w *BackupFileWriter) Seek(offset int64, whence int) (int64, error) {
	ret, err := w.s.seek(offset, whence)
	runtime.KeepAlive(w)
	return ret, err
}

func NewBackupFileWriter(path string, processSecurity, overwrite bool) (*BackupFileWriter, error) {
	st, err := newBkupStruct(path, true, processSecurity, overwrite)
	if err != nil {
		return nil, err
	}

	w := &BackupFileWriter{st}
	w.s.cleanup = runtime.AddCleanup(w, func(s *bkupStruct) { s.close() }, st)

	return w, nil
}

var (
	defaultHandler = func(ctx BackupCtx, data []byte) ([]byte, error) {
		if ctx.Hdr.IsActive() {
			hdrBuf, err := ctx.Hdr.ToBytes()
			if err != nil {
				return nil, err
			}
			return append(hdrBuf, data...), ctx.LastErr
		}
		return data, ctx.LastErr // return as-is
	}
	defaultWriteCb = func(err error) error {
		return err
	}
)

func extractStrmName(s string) string {
	n := len(s)
	if n < 1 || s[0] != ':' {
		return ""
	}

	if n < 6 || s[n-6:] != ":$DATA" {
		return ""
	}

	return s[1 : n-6]
}

// BackupHeader represents a header of backup stream of a backup file.
type BackupHeader struct {
	// indicates the type of data in this backup stream
	Id StreamType
	// indicates properties of the backup stream
	Attributes uint32
	// the length of the data portion of the backup stream, excluding the length
	// of the offset if Id == BackupSparseBlock
	Size int64
	// if Id == BackupAlternateData, name of alternate data stream, which is the
	// part of the actual stored format ":Name:$DATA"
	Name string
	// if Id == BackupSparseBlock, offset within file stream of the data contained in this sparse block
	SparseOffset uint64

	fullSize int  // size of the header including extra data
	active   bool // header is active and should be handled
}

// IsActive returns true if the header is currently active and needs to be handled.
func (hdr *BackupHeader) IsActive() bool {
	return hdr.active
}

// GetFullSize returns the size of the header including extra data.
func (hdr *BackupHeader) GetFullSize() int {
	return hdr.fullSize
}

func (hdr *BackupHeader) fill(r io.Reader) error {
	hdr.Name = ""
	hdr.SparseOffset = 0

	buf := make([]byte, hdrSz)
	_, err := io.ReadFull(r, buf)
	if err != nil {
		return err
	}

	rawHdr := (*w32api.WIN32_STREAM_ID)(unsafe.Pointer(&buf[0]))
	hdr.Id = StreamType(rawHdr.StreamId)
	hdr.Attributes = rawHdr.StreamAttributes
	hdr.Size = rawHdr.Size
	hdr.fullSize = hdrSz

	switch hdr.Id {
	case BackupAlternateData:
		if nameSize := rawHdr.StreamNameSize; nameSize != 0 {
			if nameSize&1 != 0 {
				return errors.New("length of the name is an odd number")
			}

			buf := make([]byte, nameSize)
			_, err = io.ReadFull(r, buf)
			if err != nil {
				return err
			}

			hdr.Name = extractStrmName(windows.UTF16ToString(unsafe.Slice((*uint16)(unsafe.Pointer(&buf[0])), nameSize/2)))
			hdr.fullSize += int(nameSize)
		} else {
			return ErrEmptyADSName
		}

	case BackupSparseBlock:
		var buf [offsetSz]byte
		_, err = io.ReadFull(r, buf[:])
		if err != nil {
			return err
		}

		hdr.SparseOffset = *(*uint64)(unsafe.Pointer(&buf[0]))
		runtime.KeepAlive(buf)

		hdr.Size -= offsetSz
		hdr.fullSize += offsetSz
	}

	hdr.active = true

	return nil
}

// ToBytes returns a byte slice which contains [WIN32_STREAM_ID] struct
func (hdr *BackupHeader) ToBytes() ([]byte, error) {
	rawHdr := w32api.WIN32_STREAM_ID{
		StreamId:         uint32(hdr.Id),
		StreamAttributes: hdr.Attributes,
		Size:             hdr.Size,
	}

	var extraBuf []byte // extra bytes before data region

	switch hdr.Id {
	case BackupAlternateData:
		if len(hdr.Name) == 0 {
			return nil, ErrEmptyADSName
		}

		u16Name, err := windows.UTF16FromString(":" + hdr.Name + ":$DATA")
		if err != nil {
			return nil, err
		}
		// truncate NUL character
		extraBuf = slices.Clone(unsafe.Slice((*byte)(unsafe.Pointer(&u16Name[0])), (len(u16Name)-1)*2))
		rawHdr.StreamNameSize = uint32(len(extraBuf))
	case BackupSparseBlock:
		extraBuf = slices.Clone(unsafe.Slice((*byte)(unsafe.Pointer(&hdr.SparseOffset)), offsetSz))
		runtime.KeepAlive(hdr)
	}

	hdrBuf := slices.Concat(unsafe.Slice((*byte)(unsafe.Pointer(&rawHdr)), hdrSz), extraBuf)
	runtime.KeepAlive(rawHdr)

	return hdrBuf, nil
}

type BackupCtx struct {
	Hdr       *BackupHeader
	BytesLeft int64 // bytes left in the current data stream
	LastErr   error // last error from read or write

	state uint8
}

// BackupUtil is a wrapper for backing up file with user-defined handler.
type BackupUtil struct {
	r        io.ReadSeekCloser
	ctx      BackupCtx
	leftData []byte // remaining data for read

	// handle data after reading based on the context
	//
	// if (*BackupHeader).IsActive returns true, the data of
	// the header should be handled and passed to the returned
	// slice
	Handler func(ctx BackupCtx, data []byte) ([]byte, error)
}

func (u *BackupUtil) handleRead(p []byte) (int, error) {
	readSize := min(int64(len(p)), u.ctx.BytesLeft)
	if u.ctx.Hdr.active && u.ctx.BytesLeft > int64(len(p)) {
		// reduce read size to prevent leftover data due to reading header
		// from default handler
		readSize -= int64(u.ctx.Hdr.fullSize)
	}

	var buf []byte
	var err error
	if readSize > 0 {
		var actualRead int
		actualRead, u.ctx.LastErr = u.r.Read(p[:readSize])
		buf, err = u.Handler(u.ctx, p[:actualRead])
		if err != nil {
			return 0, err
		}
		u.ctx.BytesLeft -= int64(actualRead)
	} else {
		buf, err = u.Handler(u.ctx, nil)
		if err != nil {
			return 0, err
		}
	}

	copied := copy(p, buf)
	if copied < len(buf) {
		u.leftData = buf[copied:]
	}

	return copied, nil
}

func (u *BackupUtil) Read(p []byte) (int, error) {
	var err error

	// process previous leftover data to follow up
	if len(u.leftData) != 0 {
		copied := copy(p, u.leftData)
		u.leftData = u.leftData[copied:]
		return copied, nil
	}

	// handle state
	for {
		switch u.ctx.state {
		case strmState:
			if err = u.ctx.Hdr.fill(u.r); err != nil {
				return 0, err
			}
			u.ctx.state = dataState
			u.ctx.BytesLeft = u.ctx.Hdr.Size

		case dataState:
			if u.ctx.BytesLeft == 0 {
				u.ctx.state = strmState
				continue
			}

			n, err := u.handleRead(p)
			u.ctx.Hdr.active = false
			return n, err
		}
	}
}

// Seek implements [io.Seeker].Seek, note that if [BackupFileReader] is used only
// [io.SeekCurrent] can be used for whence due to limitation of [w32api.BackupSeek].
func (u *BackupUtil) Seek(offset int64, whence int) (int64, error) {
	if u.ctx.state == strmState {
		return 0, ErrSkipHeader
	}

	var ret int64
	ret, u.ctx.LastErr = u.r.Seek(offset, whence)
	u.ctx.BytesLeft -= ret

	switch u.ctx.LastErr {
	case nil:
		return ret, nil
	case windows.ERROR_SEEK:
		// seeked to next stream header
		seeked := u.ctx.BytesLeft
		u.ctx.BytesLeft = 0
		u.ctx.state = strmState
		return seeked, nil
	default:
		return ret, u.ctx.LastErr
	}
}

func (u *BackupUtil) Close() error {
	u.leftData = nil
	return u.r.Close()
}

func NewBackupUtil(r io.ReadSeekCloser) *BackupUtil {
	return &BackupUtil{
		r:       r,
		ctx:     BackupCtx{Hdr: &BackupHeader{}},
		Handler: defaultHandler,
	}
}

// RestoreUtil is a wrapper for restoring file with user-defined handler.
type RestoreUtil struct {
	w       w32api.WriteSeekCloser
	ctx     BackupCtx
	hdrData []byte // handle header data

	// handle data before writing based on the context
	//
	// if (*BackupHeader).IsActive returns true, the data of
	// the header should be handled and passed to the returned
	// slice
	Handler func(ctx BackupCtx, data []byte) ([]byte, error)
	// callback for handling the returned error from the underlying [io.Writer]
	//
	// if not set explicitly, it just returns the error itself
	WriteCb func(err error) error
}

func (rs *RestoreUtil) handleWrite(p []byte) (int, error) {
	pLen := len(p)
	buf, err := rs.Handler(rs.ctx, p)
	if err != nil {
		return 0, err
	}

	writeSize := len(buf)
	var actualWrite, written int
	for actualWrite < writeSize {
		written, rs.ctx.LastErr = rs.w.Write(buf)
		err = rs.WriteCb(rs.ctx.LastErr)
		if err != nil {
			break
		}
		actualWrite += written
		buf = buf[written:]
	}

	if err != nil {
		return 0, err
	}

	var consumed int
	if rs.ctx.Hdr.active {
		consumed = rs.ctx.Hdr.fullSize
	}
	consumed += pLen
	rs.ctx.BytesLeft -= int64(pLen)

	return consumed, nil
}

func (rs *RestoreUtil) Write(p []byte) (int, error) {
	var err error
	var startOffset, consumed int
	inputLen := len(p)

	for consumed < inputLen {
		switch rs.ctx.state {
		case strmState:
			rs.hdrData = append(rs.hdrData, p...)
			r := bytes.NewReader(rs.hdrData)
			err = rs.ctx.Hdr.fill(r)
			if err == io.ErrUnexpectedEOF || err == io.EOF {
				// insufficient buffer, save this and get more
				return inputLen, nil
			} else if err != nil {
				return 0, err
			}
			startOffset = min(rs.ctx.Hdr.fullSize, rs.ctx.Hdr.fullSize-(len(rs.hdrData)-inputLen))
			rs.hdrData = rs.hdrData[:0] // reset header data buffer
			rs.ctx.state = dataState
			rs.ctx.BytesLeft = rs.ctx.Hdr.Size

		case dataState:
			if rs.ctx.BytesLeft == 0 {
				rs.ctx.state = strmState
				continue
			}

			endOffset := min(len(p), startOffset+int(rs.ctx.BytesLeft))
			n, err := rs.handleWrite(p[startOffset:endOffset])
			if err != nil {
				return consumed, err
			}
			consumed += n
			rs.ctx.Hdr.active = false
			if n < len(p) {
				p = p[n:]
			} else if endOffset < len(p) {
				// a data for next stream header remains, save it
				rs.hdrData = slices.Clone(p[endOffset:])
			}
		}
	}

	return min(consumed, inputLen), nil
}

// Seek implements [io.Seeker].Seek, note that if [BackupFileWriter] is used only
// [io.SeekCurrent] can be used for whence due to limitation of [w32api.BackupSeek].
func (rs *RestoreUtil) Seek(offset int64, whence int) (int64, error) {
	if rs.ctx.state == strmState {
		return 0, ErrSkipHeader
	}

	var ret int64
	ret, rs.ctx.LastErr = rs.w.Seek(offset, whence)
	rs.ctx.BytesLeft -= ret

	switch rs.ctx.LastErr {
	case nil:
		return ret, nil
	case windows.ERROR_SEEK:
		// seeked to next stream header
		seeked := rs.ctx.BytesLeft
		rs.ctx.BytesLeft = 0
		rs.ctx.state = strmState
		return seeked, nil
	default:
		return ret, rs.ctx.LastErr
	}
}

func (rs *RestoreUtil) Close() error {
	rs.hdrData = nil
	return rs.w.Close()
}

func NewRestoreUtil(w w32api.WriteSeekCloser) *RestoreUtil {
	return &RestoreUtil{
		w:       w,
		ctx:     BackupCtx{Hdr: &BackupHeader{}},
		Handler: defaultHandler,
		WriteCb: defaultWriteCb,
	}
}
