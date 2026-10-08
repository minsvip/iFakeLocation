package main

import (
	"fmt"
	"time"

	"github.com/danielpaulus/go-ios/ios"
	"github.com/danielpaulus/go-ios/ios/instruments"
	"github.com/danielpaulus/go-ios/ios/tunnel"
)

func main() {
	devices, err := ios.ListDevices()
	if err != nil || len(devices.DeviceList) == 0 {
		fmt.Printf("No devices: %v\n", err)
		return
	}
	dev := devices.DeviceList[0]
	fmt.Printf("Device: %s\n", dev.Properties.SerialNumber)

	port := 58222
	fmt.Printf("Connecting userspace tunnel on port %d...\n", port)
	tun, err := tunnel.ConnectUserSpaceTunnelLockdown(dev, port)
	if err != nil {
		fmt.Printf("ConnectUserSpaceTunnelLockdown err: %v\n", err)
		return
	}
	defer tun.Close()

	fmt.Printf("Tunnel established! Address: %s, RsdPort: %d\n", tun.Address, tun.RsdPort)

	dev.UserspaceTUN = true
	dev.UserspaceTUNHost = "127.0.0.1"
	dev.UserspaceTUNPort = port

	rsdService, err := ios.NewWithAddrPortDevice(tun.Address, tun.RsdPort, dev)
	if err != nil {
		fmt.Printf("NewWithAddrPortDevice err: %v\n", err)
		return
	}
	defer rsdService.Close()

	rsdProvider, err := rsdService.Handshake()
	if err != nil {
		fmt.Printf("RSD Handshake err: %v\n", err)
		return
	}
	fmt.Printf("RSD Handshake success! Services available.\n")

	rsdDev, err := ios.GetDeviceWithAddress(dev.Properties.SerialNumber, tun.Address, rsdProvider)
	if err != nil {
		fmt.Printf("GetDeviceWithAddress err: %v\n", err)
		return
	}
	rsdDev.UserspaceTUN = true
	rsdDev.UserspaceTUNHost = "127.0.0.1"
	rsdDev.UserspaceTUNPort = port

	fmt.Printf("SupportsRsd: %v\n", rsdDev.SupportsRsd())

	simService, err := instruments.NewLocationSimulationService(rsdDev)
	if err != nil {
		fmt.Printf("NewLocationSimulationService err: %v\n", err)
		return
	}
	defer simService.Close()

	fmt.Printf("Location simulation service created successfully!\n")
	err = simService.StartSimulateLocation(37.7749, -122.4194)
	if err != nil {
		fmt.Printf("StartSimulateLocation err: %v\n", err)
		return
	}
	fmt.Printf("Simulating location (37.7749, -122.4194) for 5 seconds...\n")
	time.Sleep(5 * time.Second)

	err = simService.StopSimulateLocation()
	fmt.Printf("StopSimulateLocation: err=%v\n", err)
}
