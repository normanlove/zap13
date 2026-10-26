package main

import (
	"fmt"

	"golang.org/x/sys/windows/registry"
)

// Windows stores the accent colour as 0xAABBGGRR
func accentColor() string {
	keys := []struct{ path, value string }{
		{`Software\Microsoft\Windows\CurrentVersion\Explorer\Accent`, "AccentColorMenu"},
		{`Software\Microsoft\Windows\DWM`, "AccentColor"},
	}
	for _, k := range keys {
		key, err := registry.OpenKey(registry.CURRENT_USER, k.path, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		v, _, err := key.GetIntegerValue(k.value)
		key.Close()
		if err == nil {
			return fmt.Sprintf("#%02x%02x%02x", v&0xff, v>>8&0xff, v>>16&0xff)
		}
	}
	return ""
}
