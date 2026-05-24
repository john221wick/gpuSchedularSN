package agent

type Vendor int

const (
	Unknown Vendor = iota
	NVIDIA
	AMD
	Intel
	Apple
)

func (v Vendor) String() string {
	switch v {
	case NVIDIA:
		return "NVIDIA"
	case AMD:
		return "AMD"
	case Intel:
		return "Intel"
	case Apple:
		return "Apple"
	default:
		return "Unknown"
	}
}

type LinkType int

const (
	LinkNone LinkType = iota
	PCIe
	NVLink
	NVSwitch
	XGMI
	XeLink
	Thunderbolt
	UnifiedMemory
)

func (l LinkType) String() string {
	switch l {
	case PCIe:
		return "PCIe"
	case NVLink:
		return "NVLink"
	case NVSwitch:
		return "NVSwitch"
	case XGMI:
		return "XGMI"
	case XeLink:
		return "XeLink"
	case Thunderbolt:
		return "Thunderbolt"
	case UnifiedMemory:
		return "UnifiedMemory"
	default:
		return "None"
	}
}

type GPUDevice struct {
	ID             int
	Vendor         Vendor
	Name           string
	VRAMTotalMB    uint64
	VRAMUsedMB     uint64
	UtilizationPct float32
	TemperatureC   int
	VendorIndex    int
}

type GPULink struct {
	GPUA          int
	GPUB          int
	Type          LinkType
	BandwidthGBps float32
}
