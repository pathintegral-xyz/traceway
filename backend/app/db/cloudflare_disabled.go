//go:build !cloudflare

package db

func IsCloudflare() bool { return false }
