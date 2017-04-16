package instance

import "github.com/denverdino/aliyungo/metadata"

func getMetadata() *metadata.MetaData {
	return metadata.NewMetaData(nil)
}

// GetRegion returns the Aliyun region this instance is in.
func GetRegion() (string, error) {
	return getMetadata().Region()
}
