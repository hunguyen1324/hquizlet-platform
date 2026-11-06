//go:build !linux

package http

// Production runs Linux. Other platforms still enforce file-size and remote capacity limits.
func videoDiskFree(string) (uint64, error) { return ^uint64(0), nil }
