//go:build windows

package abocado

import (
	"fmt"
	"syscall"
	"unsafe"
)

type winDataBlob struct {
	Size uint32
	Data *byte
}

var crypt32 = syscall.NewLazyDLL("Crypt32.dll")
var kernel32 = syscall.NewLazyDLL("Kernel32.dll")

func dpapi(input []byte, protect bool) ([]byte, error) {
	if len(input) == 0 {
		return nil, nil
	}
	in := winDataBlob{Size: uint32(len(input)), Data: &input[0]}
	var out winDataBlob
	proc := crypt32.NewProc("CryptProtectData")
	if !protect {
		proc = crypt32.NewProc("CryptUnprotectData")
	}
	r, _, callErr := proc.Call(uintptr(unsafe.Pointer(&in)), 0, 0, 0, 0, 1, uintptr(unsafe.Pointer(&out)))
	if r == 0 {
		return nil, fmt.Errorf("Windows DPAPI failed: %w", callErr)
	}
	defer kernel32.NewProc("LocalFree").Call(uintptr(unsafe.Pointer(out.Data)))
	return append([]byte(nil), unsafe.Slice(out.Data, out.Size)...), nil
}

func protectCurrentUser(data []byte) ([]byte, error)   { return dpapi(data, true) }
func unprotectCurrentUser(data []byte) ([]byte, error) { return dpapi(data, false) }

