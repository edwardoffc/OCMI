package internal

import (
	"os"

	"golang.org/x/sys/windows"
)

func maintenanceMarkSparse(f *os.File) error {
	var returned uint32
	return windows.DeviceIoControl(windows.Handle(f.Fd()), windows.FSCTL_SET_SPARSE, nil, 0, nil, 0, &returned, nil)
}
