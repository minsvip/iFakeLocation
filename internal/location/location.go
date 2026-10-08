package location

import (
	"fmt"
	"log"
	"net"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/danielpaulus/go-ios/ios"
	"github.com/danielpaulus/go-ios/ios/amfi"
	"github.com/danielpaulus/go-ios/ios/imagemounter"
	"github.com/danielpaulus/go-ios/ios/instruments"
	"github.com/danielpaulus/go-ios/ios/simlocation"
	"github.com/danielpaulus/go-ios/ios/tunnel"

	"ifakelocation/internal/imagehelper"
	"ifakelocation/internal/models"
)

type activeSession struct {
	tunnel     tunnel.Tunnel
	rsdService ios.RsdService
	simService *instruments.LocationSimulationService
}

var (
	sessionsMu     sync.Mutex
	activeSessions = make(map[string]*activeSession)
)

// CloseAllSessions shuts down all active tunnel and location sessions
func CloseAllSessions() {
	sessionsMu.Lock()
	defer sessionsMu.Unlock()

	for udid, s := range activeSessions {
		log.Printf("Closing simulation session for %s...", udid)
		if s.simService != nil {
			_ = s.simService.StopSimulateLocation()
			s.simService.Close()
		}
		s.rsdService.Close()
		_ = s.tunnel.Close()
		delete(activeSessions, udid)
	}
}

// getFreePort finds an unused TCP port on localhost
func getFreePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func isDeviceIOS17Plus(entry ios.DeviceEntry, devInfo *models.DeviceInformation) bool {
	v, err := ios.GetProductVersion(entry)
	if err == nil && v != nil && v.Major() >= 17 {
		return true
	}
	if devInfo != nil && devInfo.ProductVersion != "" {
		parts := strings.Split(devInfo.ProductVersion, ".")
		if m, err := strconv.Atoi(parts[0]); err == nil && m >= 17 {
			return true
		}
	}
	return false
}

// EnsureDeveloperModeAndMount mounts the developer image if not already mounted
func EnsureDeveloperModeAndMount(entry ios.DeviceEntry, devInfo *models.DeviceInformation) error {
	version, err := ios.GetProductVersion(entry)
	if err == nil && version.Major() >= 16 {
		isDev, devErr := imagemounter.IsDevModeEnabled(entry)
		if devErr == nil && !isDev {
			conn, aErr := amfi.New(entry)
			if aErr == nil {
				_ = conn.RevealDevMode()
				conn.Close()
			}
			return fmt.Errorf("Please turn on Developer Mode first via Settings >> Privacy & Security on your device.")
		}
	}

	hasImages, paths, _ := imagehelper.HasImageForDevice(devInfo)
	if !hasImages || len(paths) == 0 {
		return fmt.Errorf("The developer images for the specified device are missing.")
	}

	mounter, err := imagemounter.NewImageMounter(entry)
	if err != nil {
		return fmt.Errorf("failed connecting to image mounter: %w", err)
	}
	defer mounter.Close()

	images, err := mounter.ListImages()
	if err != nil || len(images) == 0 {
		majorVer := 0
		if version != nil {
			majorVer = int(version.Major())
		} else {
			parts := strings.Split(devInfo.ProductVersion, ".")
			if len(parts) > 0 {
				majorVer, _ = strconv.Atoi(parts[0])
			}
		}

		mountPath := paths[0]
		if majorVer >= 17 {
			mountPath = filepath.Dir(paths[0])
		}
		if err := mounter.MountImage(mountPath); err != nil {
			if strings.Contains(err.Error(), "DeviceLocked") {
				return fmt.Errorf("Please unlock your iOS device screen and try again.")
			}
			return fmt.Errorf("failed to mount developer image: %w", err)
		}
	}

	return nil
}

// setLocationIOS17 simulates location using Userspace Tunnel and DTX LocationSimulationService
func setLocationIOS17(entry ios.DeviceEntry, udid string, lat float64, lng float64) error {
	sessionsMu.Lock()
	defer sessionsMu.Unlock()

	// 1. Try to update an existing session if active
	if session, ok := activeSessions[udid]; ok {
		err := session.simService.StartSimulateLocation(lat, lng)
		if err == nil {
			log.Printf("Updated simulated location for %s to (%.6f, %.6f)", udid, lat, lng)
			return nil
		}
		// Connection lost; clean up and recreate
		log.Printf("Existing session failed (%v), recreating...", err)
		session.simService.Close()
		session.rsdService.Close()
		_ = session.tunnel.Close()
		delete(activeSessions, udid)
	}

	// 2. Obtain a free port for Userspace TUN
	tunPort, err := getFreePort()
	if err != nil {
		return fmt.Errorf("failed allocating port for userspace tunnel: %w", err)
	}

	// 3. Connect userspace tunnel over lockdown
	tun, err := tunnel.ConnectUserSpaceTunnelLockdown(entry, tunPort)
	if err != nil {
		if strings.Contains(err.Error(), "DeviceLocked") {
			return fmt.Errorf("Please unlock your iOS device screen and try again.")
		}
		return fmt.Errorf("failed establishing userspace tunnel: %w", err)
	}

	// Configure entry for RSD connection
	entry.UserspaceTUN = true
	entry.UserspaceTUNHost = "127.0.0.1"
	entry.UserspaceTUNPort = tunPort

	// 4. Connect to RSD service
	rsdService, err := ios.NewWithAddrPortDevice(tun.Address, tun.RsdPort, entry)
	if err != nil {
		_ = tun.Close()
		return fmt.Errorf("failed connecting to RSD on device: %w", err)
	}

	rsdProvider, err := rsdService.Handshake()
	if err != nil {
		rsdService.Close()
		_ = tun.Close()
		return fmt.Errorf("RSD handshake failed: %w", err)
	}

	rsdDev, err := ios.GetDeviceWithAddress(udid, tun.Address, rsdProvider)
	if err != nil {
		rsdService.Close()
		_ = tun.Close()
		return fmt.Errorf("failed getting device with address: %w", err)
	}
	rsdDev.UserspaceTUN = true
	rsdDev.UserspaceTUNHost = "127.0.0.1"
	rsdDev.UserspaceTUNPort = tunPort

	// 5. Connect to LocationSimulationService via instruments
	simService, err := instruments.NewLocationSimulationService(rsdDev)
	if err != nil {
		rsdService.Close()
		_ = tun.Close()
		return fmt.Errorf("failed initializing location simulation service: %w", err)
	}

	// 6. Simulate location
	if err := simService.StartSimulateLocation(lat, lng); err != nil {
		simService.Close()
		rsdService.Close()
		_ = tun.Close()
		return fmt.Errorf("failed starting location simulation: %w", err)
	}

	activeSessions[udid] = &activeSession{
		tunnel:     tun,
		rsdService: rsdService,
		simService: simService,
	}

	log.Printf("Successfully simulated location on iOS 17+ device %s to (%.6f, %.6f)", udid, lat, lng)
	return nil
}

// stopLocationIOS17 stops simulated location on iOS 17+
func stopLocationIOS17(udid string) error {
	sessionsMu.Lock()
	defer sessionsMu.Unlock()

	session, ok := activeSessions[udid]
	if !ok {
		return nil
	}

	log.Printf("Stopping simulation session for %s...", udid)
	_ = session.simService.StopSimulateLocation()
	session.simService.Close()
	session.rsdService.Close()
	_ = session.tunnel.Close()
	delete(activeSessions, udid)
	return nil
}

// SetLocation sets simulated GPS coordinates for a connected iOS device
func SetLocation(udid string, lat float64, lng float64, devInfo *models.DeviceInformation) error {
	entry, err := ios.GetDevice(udid)
	if err != nil {
		return fmt.Errorf("Unable to find the specified device. Are you sure it is connected?: %w", err)
	}

	if err := EnsureDeveloperModeAndMount(entry, devInfo); err != nil {
		return err
	}

	if isDeviceIOS17Plus(entry, devInfo) {
		return setLocationIOS17(entry, udid, lat, lng)
	}

	latStr := fmt.Sprintf("%.7f", lat)
	lngStr := fmt.Sprintf("%.7f", lng)

	if err := simlocation.SetLocation(entry, latStr, lngStr); err != nil {
		return fmt.Errorf("Failed to set simulated location: %w", err)
	}

	return nil
}

// StopLocation resets the device's location to real GPS
func StopLocation(udid string, devInfo *models.DeviceInformation) error {
	entry, err := ios.GetDevice(udid)
	if err != nil {
		return fmt.Errorf("Unable to find the specified device. Are you sure it is connected?: %w", err)
	}

	if err := EnsureDeveloperModeAndMount(entry, devInfo); err != nil {
		return err
	}

	if isDeviceIOS17Plus(entry, devInfo) {
		return stopLocationIOS17(udid)
	}

	if err := simlocation.ResetLocation(entry); err != nil {
		return fmt.Errorf("Failed to stop simulated location: %w", err)
	}

	return nil
}
