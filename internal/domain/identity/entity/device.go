package entity

type DeviceType string

const (
	DeviceTypeWeb DeviceType = "web"
)

var validDeviceTypes = map[DeviceType]bool{
	DeviceTypeWeb: true,
}

func CheckDeviceType(s string) bool {
	_, ok := validDeviceTypes[DeviceType(s)]
	return ok
}
