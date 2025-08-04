package types

// GetBytes is a helper for serialising
func (p OracleRequestPacketData) GetBytes() ([]byte, error) {
	var modulePacket PricefeedPacketData

	modulePacket.Packet = &PricefeedPacketData_OracleRequestPacket{&p}

	return modulePacket.Marshal()
}
