package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

func main() {
	// 1. Get devices
	resp, err := http.Get("http://localhost:49215/get_devices")
	if err != nil {
		fmt.Printf("get_devices err: %v\n", err)
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("get_devices: %s\n", string(body))

	var devices []struct {
		UDID        string `json:"udid"`
		DisplayName string `json:"display_name"`
	}
	if err := json.Unmarshal(body, &devices); err != nil || len(devices) == 0 {
		fmt.Println("No devices found from API")
		return
	}

	udid := devices[0].UDID
	fmt.Printf("Using device UDID: %s (%s)\n", udid, devices[0].DisplayName)

	// 2. Set location (e.g. San Francisco)
	setReq := map[string]interface{}{
		"udid": udid,
		"lat":  37.774929,
		"lng":  -122.419416,
	}
	setJson, _ := json.Marshal(setReq)
	fmt.Println("Sending set_location...")
	setResp, err := http.Post("http://localhost:49215/set_location", "application/json", bytes.NewBuffer(setJson))
	if err != nil {
		fmt.Printf("set_location err: %v\n", err)
		return
	}
	defer setResp.Body.Close()
	setBody, _ := io.ReadAll(setResp.Body)
	fmt.Printf("set_location response: %s\n", string(setBody))

	time.Sleep(3 * time.Second)

	// 3. Stop location
	stopReq := map[string]interface{}{
		"udid": udid,
	}
	stopJson, _ := json.Marshal(stopReq)
	fmt.Println("Sending stop_location...")
	stopResp, err := http.Post("http://localhost:49215/stop_location", "application/json", bytes.NewBuffer(stopJson))
	if err != nil {
		fmt.Printf("stop_location err: %v\n", err)
		return
	}
	defer stopResp.Body.Close()
	stopBody, _ := io.ReadAll(stopResp.Body)
	fmt.Printf("stop_location response: %s\n", string(stopBody))
}
