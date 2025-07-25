package ea

import (
	"github.com/go-sw/winfs/w32api"
)

const (
	NeedEa = w32api.FILE_NEED_EA

	fullInfoHeaderSize = 8 // 4 + 1 + 1 + 2
	getInfoHeaderSize  = 5 // 4 + 1
)

// EaInfo is a simplified struct of FILE_FULL_EA_INFORMATION, see https://learn.microsoft.com/en-us/windows-hardware/drivers/ddi/wdm/ns-wdm-_file_full_ea_information
type EaInfo struct {
	Flags   uint8
	EaName  string
	EaValue []byte
}
