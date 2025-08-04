package types

// GetBytes is a helper for serialising
func (p OracleResponsePacketData) GetBytes() ([]byte, error) {
	var modulePacket PricefeedPacketData

	modulePacket.Packet = &PricefeedPacketData_OracleResponsePacket{&p}

	return modulePacket.Marshal()
}
