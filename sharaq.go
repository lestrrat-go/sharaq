package sharaq

import "hash/crc64"

var crc64Table *crc64.Table

func init() {
	crc64Table = crc64.MakeTable(crc64.ISO)
}
