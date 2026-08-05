package linux

func Align(length uint32, sectorSize uint32) uint32 {
	if sectorSize == 0 {
		return length
	}
	remainder := length % sectorSize
	if remainder == 0 {
		return length
	}
	return length + sectorSize - remainder
}
