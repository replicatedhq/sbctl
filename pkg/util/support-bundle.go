package util

var (
	// sbResourceCompatibilityMap
	sbResourceCompatibilityMap = map[string]string{
		"persistentvolumeclaims": "pvcs",
		"persistentvolumes":      "pvs",
		"storageclasses":         "storage-classes",
	}
)

// GetSBCompatibleResourceName returns SupportBundle compatible resource name if exists else the same resource name
func GetSBCompatibleResourceName(resource string) string {
	if val, ok := sbResourceCompatibilityMap[resource]; ok {
		return val
	}
	return resource
}
