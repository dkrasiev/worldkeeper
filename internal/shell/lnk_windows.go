package shell

import (
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// A .lnk file is written and read through the shell's own IShellLink COM
// object, the way Explorer makes shortcuts, called straight through its
// method table (no cgo). Only the methods used here are listed.

var (
	ole32                = windows.NewLazySystemDLL("ole32.dll")
	procCoCreateInstance = ole32.NewProc("CoCreateInstance")
	procPropVariantClear = ole32.NewProc("PropVariantClear")

	clsidShellLink   = windows.GUID{Data1: 0x00021401, Data4: [8]byte{0xc0, 0, 0, 0, 0, 0, 0, 0x46}}
	iidShellLinkW    = windows.GUID{Data1: 0x000214f9, Data4: [8]byte{0xc0, 0, 0, 0, 0, 0, 0, 0x46}}
	iidPersistFile   = windows.GUID{Data1: 0x0000010b, Data4: [8]byte{0xc0, 0, 0, 0, 0, 0, 0, 0x46}}
	iidPropertyStore = windows.GUID{Data1: 0x886d8eeb, Data2: 0x8cf2, Data3: 0x4446, Data4: [8]byte{0x8d, 0x02, 0xcd, 0xba, 0x1d, 0xbd, 0xcf, 0x99}}
	// PKEY_AppUserModel_ID
	pkeyAppUserModelID = propertyKey{
		fmtid: windows.GUID{Data1: 0x9f4c2855, Data2: 0x9f79, Data3: 0x4b39, Data4: [8]byte{0xa8, 0xd0, 0xe1, 0xd4, 0x2d, 0xe1, 0xd5, 0xf3}},
		pid:   5,
	}
)

// Method indexes in the COM method tables (vtables).
const (
	// IUnknown
	mQueryInterface = 0
	mRelease        = 2
	// IShellLinkW
	mGetPath             = 3
	mGetDescription      = 6
	mSetDescription      = 7
	mGetWorkingDirectory = 8
	mSetWorkingDirectory = 9
	mGetArguments        = 10
	mSetArguments        = 11
	mSetPath             = 20
	// IPersistFile
	mLoad = 5
	mSave = 6
	// IPropertyStore
	mGetValue = 5
	mSetValue = 6
	mCommit   = 7
)

const (
	clsctxInprocServer = 0x1
	slgpRawPath        = 0x4 // GetPath: the path as stored
	stgmRead           = 0x0
	vtLPWSTR           = 31
	// CoInitializeEx results: COM was already set up on this thread, as
	// single-threaded (fine, but still to be undone) or as multithreaded
	// (fine, and not ours to undo).
	sFalse          = 1
	rpcEChangedMode = 0x80010106
)

// comObject is a COM interface pointer: a pointer to its method table.
type comObject struct{ vtbl *[32]uintptr }

type propertyKey struct {
	fmtid windows.GUID
	pid   uint32
}

// propVariant is a PROPVARIANT holding a string (VT_LPWSTR).
type propVariant struct {
	vt  uint16
	_   [3]uint16
	val *uint16
	_   uintptr // the rest of the value union
}

func hresult(r uintptr) error {
	if int32(r) < 0 {
		return windows.Errno(uint32(r)) // an HRESULT is 32 bits
	}
	return nil
}

func (o *comObject) release() {
	syscall.SyscallN(o.vtbl[mRelease], uintptr(unsafe.Pointer(o)))
}

func (o *comObject) query(iid *windows.GUID) (*comObject, error) {
	var out *comObject
	r, _, _ := syscall.SyscallN(o.vtbl[mQueryInterface], uintptr(unsafe.Pointer(o)),
		uintptr(unsafe.Pointer(iid)), uintptr(unsafe.Pointer(&out)))
	return out, hresult(r)
}

// setString calls a method that takes one string.
func (o *comObject) setString(method int, s string) error {
	p, err := windows.UTF16PtrFromString(s)
	if err != nil {
		return err
	}
	r, _, _ := syscall.SyscallN(o.vtbl[method], uintptr(unsafe.Pointer(o)), uintptr(unsafe.Pointer(p)))
	return hresult(r)
}

// getString calls a method that fills a buffer: GetPath, GetArguments, …
// (extra are arguments after the buffer and its length).
func (o *comObject) getString(method int, extra ...uintptr) (string, error) {
	buf := make([]uint16, 32*1024)
	args := append([]uintptr{uintptr(unsafe.Pointer(o)), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf))}, extra...)
	r, _, _ := syscall.SyscallN(o.vtbl[method], args...)
	runtime.KeepAlive(buf)
	if err := hresult(r); err != nil {
		return "", err
	}
	return windows.UTF16ToString(buf), nil
}

// withShellLink runs fn with a new IShellLink object, on a thread with COM
// set up.
func withShellLink(fn func(link *comObject) error) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	err := windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED)
	var hr uint32
	if e, ok := err.(windows.Errno); ok {
		hr = uint32(e)
	}
	switch {
	case err == nil || hr == sFalse:
		defer windows.CoUninitialize()
	case hr == rpcEChangedMode:
	default:
		return err
	}
	var link *comObject
	r, _, _ := procCoCreateInstance.Call(uintptr(unsafe.Pointer(&clsidShellLink)), 0, clsctxInprocServer,
		uintptr(unsafe.Pointer(&iidShellLinkW)), uintptr(unsafe.Pointer(&link)))
	if err := hresult(r); err != nil {
		return err
	}
	defer link.release()
	return fn(link)
}

// writeShortcut creates or replaces the .lnk file at path.
func writeShortcut(path string, s shortcut) error {
	return withShellLink(func(link *comObject) error {
		for _, set := range []struct {
			method int
			value  string
		}{
			{mSetPath, s.Target},
			{mSetArguments, windows.ComposeCommandLine(s.Args)},
			{mSetWorkingDirectory, s.Dir},
			{mSetDescription, s.Description},
		} {
			if err := link.setString(set.method, set.value); err != nil {
				return err
			}
		}
		if s.AppID != "" {
			if err := setAppID(link, s.AppID); err != nil {
				return err
			}
		}
		file, err := link.query(&iidPersistFile)
		if err != nil {
			return err
		}
		defer file.release()
		p, err := windows.UTF16PtrFromString(path)
		if err != nil {
			return err
		}
		r, _, _ := syscall.SyscallN(file.vtbl[mSave], uintptr(unsafe.Pointer(file)), uintptr(unsafe.Pointer(p)), 1)
		return hresult(r)
	})
}

func setAppID(link *comObject, id string) error {
	store, err := link.query(&iidPropertyStore)
	if err != nil {
		return err
	}
	defer store.release()
	p, err := windows.UTF16PtrFromString(id)
	if err != nil {
		return err
	}
	v := propVariant{vt: vtLPWSTR, val: p}
	r, _, _ := syscall.SyscallN(store.vtbl[mSetValue], uintptr(unsafe.Pointer(store)),
		uintptr(unsafe.Pointer(&pkeyAppUserModelID)), uintptr(unsafe.Pointer(&v)))
	if err := hresult(r); err != nil {
		return err
	}
	r, _, _ = syscall.SyscallN(store.vtbl[mCommit], uintptr(unsafe.Pointer(store)))
	return hresult(r)
}

// readShortcut reads the .lnk file at path.
func readShortcut(path string) (shortcut, error) {
	var s shortcut
	err := withShellLink(func(link *comObject) error {
		file, err := link.query(&iidPersistFile)
		if err != nil {
			return err
		}
		defer file.release()
		p, err := windows.UTF16PtrFromString(path)
		if err != nil {
			return err
		}
		r, _, _ := syscall.SyscallN(file.vtbl[mLoad], uintptr(unsafe.Pointer(file)), uintptr(unsafe.Pointer(p)), stgmRead)
		if err := hresult(r); err != nil {
			return err
		}
		if s.Target, err = link.getString(mGetPath, 0, slgpRawPath); err != nil {
			return err
		}
		args, err := link.getString(mGetArguments)
		if err != nil {
			return err
		}
		if args != "" {
			if s.Args, err = windows.DecomposeCommandLine(args); err != nil {
				return err
			}
		}
		if s.Dir, err = link.getString(mGetWorkingDirectory); err != nil {
			return err
		}
		if s.Description, err = link.getString(mGetDescription); err != nil {
			return err
		}
		s.AppID, err = getAppID(link)
		return err
	})
	return s, err
}

func getAppID(link *comObject) (string, error) {
	store, err := link.query(&iidPropertyStore)
	if err != nil {
		return "", err
	}
	defer store.release()
	var v propVariant
	r, _, _ := syscall.SyscallN(store.vtbl[mGetValue], uintptr(unsafe.Pointer(store)),
		uintptr(unsafe.Pointer(&pkeyAppUserModelID)), uintptr(unsafe.Pointer(&v)))
	if err := hresult(r); err != nil {
		return "", err
	}
	defer procPropVariantClear.Call(uintptr(unsafe.Pointer(&v)))
	if v.vt != vtLPWSTR {
		return "", nil
	}
	return windows.UTF16PtrToString(v.val), nil
}
