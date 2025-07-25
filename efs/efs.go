//go:build windows

package efs

import (
	"errors"
	"io"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/go-sw/winfs/w32api"
)

type ExportContext[T any] struct {
	Context T

	// If the function succeeds, it must set the return value to ERROR_SUCCESS.
	Handler func(data *byte, ctx *ExportContext[T], length uint32) uintptr
}

type ImportContext[T any] struct {
	Context T

	// If the function succeeds, it must set the return value to ERROR_SUCCESS, and set the value pointed to by the length parameter to the number of bytes copied into data.
	//
	// When the end of the backup file is reached, the referenced value of length should to set to zero to tell the system that the entire file has been processed.
	Handler func(data *byte, ctx *ImportContext[T], length *uint32) uintptr // user defined callback
}

// newReadCallback creates callback function pointer to be used by ReadEncryptedFileRaw.
// Note that only a limited number of callbacks may be created in a single Go process.
func newReadCallback[T any]() uintptr {
	cb := func(data *byte, callbackContext unsafe.Pointer, length uint32) uintptr {
		c := (*ExportContext[T])(callbackContext)

		return c.Handler(data, c, length)
	}
	return windows.NewCallback(cb)
}

// newWriteCallback creates callback function pointer to be used by WriteEncryptedFileRaw.
// Note that only a limited number of callbacks may be created in a single Go process.
func newWriteCallback[T any]() uintptr {
	cb := func(data *byte, callbackContext unsafe.Pointer, length *uint32) uintptr {
		c := (*ImportContext[T])(callbackContext)

		return c.Handler(data, c, length)
	}
	return windows.NewCallback(cb)
}

type EfsClient[T, U any] struct {
	readcb, writeCb uintptr

	ReadCtx  *ExportContext[T]
	WriteCtx *ImportContext[U]

	// TODO: handle certificate(wincrypt)
}

func NewEfsClient[T, U any](readCtx T, writeCtx U) (*EfsClient[T, U], error) {
	client := &EfsClient[T, U]{
		ReadCtx: &ExportContext[T]{
			Context: readCtx,
		},
		WriteCtx: &ImportContext[U]{
			Context: writeCtx,
		},
	}

	client.readcb = newReadCallback[T]()
	client.writeCb = newWriteCallback[U]()

	return client, nil
}

type rawReadWriteCtx struct {
	target io.ReadWriter
	err    error
}

// RawReadWriter converts encrypted file or directory into a raw data keeping its encryption.
type RawReadWriter struct {
	*EfsClient[*rawReadWriteCtx, *rawReadWriteCtx]
}

func NewRawReadWriter() (*RawReadWriter, error) {
	ctx := &rawReadWriteCtx{}
	client, err := NewEfsClient(ctx, ctx)
	if err != nil {
		return nil, err
	}

	rw := &RawReadWriter{
		EfsClient: client,
	}

	rw.ReadCtx.Handler = func(data *byte, ctx *ExportContext[*rawReadWriteCtx], length uint32) uintptr {
		c := ctx.Context

		buf := unsafe.Slice(data, length)
		_, err := c.target.Write(buf)
		if err != nil {
			c.err = err
			return uintptr(windows.ERROR_WRITE_FAULT)
		}

		return uintptr(windows.ERROR_SUCCESS)
	}
	rw.WriteCtx.Handler = func(data *byte, ctx *ImportContext[*rawReadWriteCtx], length *uint32) uintptr {
		c := ctx.Context

		buf := unsafe.Slice(data, *length)
		n, err := c.target.Read(buf)
		if err == io.EOF {
			*length = 0
		} else if err != nil {
			c.err = err
			return uintptr(windows.ERROR_READ_FAULT)
		} else {
			*length = uint32(n)
		}

		return uintptr(windows.ERROR_SUCCESS)
	}

	return rw, nil
}

// ReadRaw reads encrypted file as a raw stream then writes it to dst.
func (rw *RawReadWriter) ReadRaw(srcFile string, dst io.ReadWriter) error {
	stat, err := os.Stat(srcFile)
	if err != nil {
		return err
	}

	var openFlag uint32 = 0
	if stat.IsDir() {
		openFlag |= w32api.CREATE_FOR_DIR
	}

	rw.ReadCtx.Context.target = dst

	rawCtx, err := w32api.OpenEncryptedFileRaw(srcFile, openFlag)
	if err != nil {
		return err
	}
	defer w32api.CloseEncryptedFileRaw(rawCtx)

	err = w32api.ReadEncryptedFileRaw(rw.readcb, unsafe.Pointer(rw.ReadCtx), rawCtx)
	if err != nil {
		return errors.Join(err, rw.ReadCtx.Context.err)
	}

	return nil
}

// WriteRaw reads data from src then writes it as a encrypted file,
// if dir is true data is written as a directory with encrypted files.
func (rw *RawReadWriter) WriteRaw(dstFile string, src io.ReadWriter, dir bool) error {
	var openFlag uint32 = w32api.CREATE_FOR_IMPORT
	if dir {
		openFlag |= w32api.CREATE_FOR_DIR
	}

	rw.WriteCtx.Context.target = src

	rawCtx, err := w32api.OpenEncryptedFileRaw(dstFile, openFlag)
	if err != nil {
		return err
	}
	defer w32api.CloseEncryptedFileRaw(rawCtx)

	err = w32api.WriteEncryptedFileRaw(rw.writeCb, unsafe.Pointer(rw.WriteCtx), rawCtx)
	if err != nil {
		return errors.Join(err, rw.WriteCtx.Context.err)
	}

	return nil
}
