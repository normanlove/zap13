package main

import (
	"context"
	"encoding/json"
	"os/exec"
	"syscall"
	"time"
	"unsafe"
)

type status struct {
	AC         bool    `json:"ac"`
	Percent    *int    `json:"percent"`
	ChargeW    float64 `json:"chargeW"`
	DischargeW float64 `json:"dischargeW"`
	Source     string  `json:"source"`
	Limits     *limits `json:"limits"`
	Manual     bool    `json:"manual"`
}

type sysPowerStatus struct {
	ACLineStatus        byte
	BatteryFlag         byte
	BatteryLifePercent  byte
	SystemStatusFlag    byte
	BatteryLifeTime     uint32
	BatteryFullLifeTime uint32
}

// WMI is the easiest place to get the actual charge/discharge rate in mW
const batteryQuery = `Get-CimInstance -Namespace root/wmi -ClassName BatteryStatus | ` +
	`Select-Object -First 1 ChargeRate,DischargeRate | ConvertTo-Json -Compress`

func readPower() status {
	var ps sysPowerStatus
	procGetSystemPowerStatus.Call(uintptr(unsafe.Pointer(&ps)))

	st := status{AC: ps.ACLineStatus == 1}
	if ps.BatteryLifePercent != 255 {
		p := int(ps.BatteryLifePercent)
		st.Percent = &p
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command", batteryQuery)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000} // CREATE_NO_WINDOW
	out, err := cmd.Output()
	if err != nil {
		return st
	}
	var rate struct{ ChargeRate, DischargeRate float64 }
	if json.Unmarshal(out, &rate) == nil {
		st.ChargeW = rate.ChargeRate / 1000
		st.DischargeW = rate.DischargeRate / 1000
	}
	return st
}
