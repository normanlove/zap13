package main

import (
	"encoding/binary"

	"golang.org/x/sys/windows"
)

// ASUS ACPI interface, same one Armoury Crate and G-Helper use.
// Device IDs are from G-Helper's AsusACPI.cs.
const (
	ioctlATK = 0x0022240C
	DEVS     = 0x53564544 // set
	DSTS     = 0x53545344 // get

	devPerfMode = 0x00120075
	devPPTSlow  = 0x001200A0 // sPPT
	devPPTSPL   = 0x001200A3 // SPL
	devPPTFast  = 0x001200C1 // fPPT
	devCharger  = 0x0012006C

	modeBalanced = 0
	modeTurbo    = 1
	modeSilent   = 2
)

type atk struct {
	h windows.Handle
}

func openATK() (*atk, error) {
	name, _ := windows.UTF16PtrFromString(`\\.\ATKACPI`)
	h, err := windows.CreateFile(name,
		windows.GENERIC_READ|windows.GENERIC_WRITE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return nil, err
	}
	return &atk{h}, nil
}

func (a *atk) call(method, dev, arg uint32) (int32, error) {
	in := make([]byte, 16)
	binary.LittleEndian.PutUint32(in[0:], method)
	binary.LittleEndian.PutUint32(in[4:], 8)
	binary.LittleEndian.PutUint32(in[8:], dev)
	binary.LittleEndian.PutUint32(in[12:], arg)

	out := make([]byte, 16)
	var n uint32
	err := windows.DeviceIoControl(a.h, ioctlATK, &in[0], uint32(len(in)), &out[0], uint32(len(out)), &n, nil)
	return int32(binary.LittleEndian.Uint32(out)), err
}

// set returns the firmware's result, 1 means it accepted the value
func (a *atk) set(dev, val uint32) int32 {
	r, _ := a.call(DEVS, dev, val)
	return r
}

func (a *atk) get(dev uint32) (int32, error) {
	return a.call(DSTS, dev, 0)
}
