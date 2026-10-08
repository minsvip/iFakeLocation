package server

import (
	"embed"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"ifakelocation/internal/imagehelper"
	"ifakelocation/internal/location"
	"ifakelocation/internal/models"
)

type Server struct {
	embeddedFS embed.FS
	devicesMu  sync.RWMutex
	devices    []models.DeviceInformation
	version    string
}

func NewServer(fs embed.FS, version string) *Server {
	return &Server{
		embeddedFS: fs,
		version:    version,
	}
}

func (s *Server) findDevice(udid string) *models.DeviceInformation {
	s.devicesMu.RLock()
	for _, d := range s.devices {
		if d.UDID == udid {
			dev := d
			s.devicesMu.RUnlock()
			return &dev
		}
	}
	s.devicesMu.RUnlock()

	// If not found in cache, auto-refresh devices
	devs, err := models.GetDevices(true)
	if err == nil {
		s.devicesMu.Lock()
		s.devices = devs
		s.devicesMu.Unlock()
		for _, d := range devs {
			if d.UDID == udid {
				dev := d
				return &dev
			}
		}
	}
	return nil
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/version", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte(s.version))
	})

	mux.HandleFunc("/home_country", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		country := detectHomeCountry()
		_, _ = w.Write([]byte(country))
	})

	mux.HandleFunc("/get_devices", func(w http.ResponseWriter, r *http.Request) {
		devs, err := models.GetDevices(true)
		if err != nil {
			writeJSON(w, map[string]string{
				"error": "Unable to retrieve connected devices. Ensure iTunes or Apple Devices is installed and can detect your device(s).",
			})
			return
		}

		s.devicesMu.Lock()
		s.devices = devs
		s.devicesMu.Unlock()

		type DeviceResponse struct {
			Name        string `json:"name"`
			DisplayName string `json:"display_name"`
			UDID        string `json:"udid"`
		}

		resp := make([]DeviceResponse, len(devs))
		for i, d := range devs {
			resp[i] = DeviceResponse{
				Name:        d.Name,
				DisplayName: d.DisplayName,
				UDID:        d.UDID,
			}
		}

		writeJSON(w, resp)
	})

	mux.HandleFunc("/has_dependencies", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			UDID string `json:"udid"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, map[string]string{"error": "Invalid request body."})
			return
		}

		device := s.findDevice(req.UDID)
		if device == nil {
			writeJSON(w, map[string]string{
				"error": "Unable to find the specified device. Are you sure it is connected?",
			})
			return
		}

		hasDeps, _, verStr := imagehelper.HasImageForDevice(device)
		if !hasDeps {
			links := imagehelper.GetLinksForDevice(device)
			if len(links) == 0 {
				writeJSON(w, map[string]string{
					"error": "Your device's iOS version is not supported at this time.",
				})
				return
			}
			imagehelper.StartDownload(verStr, links)
		}

		writeJSON(w, map[string]interface{}{
			"result":  hasDeps,
			"version": verStr,
		})
	})

	mux.HandleFunc("/get_progress", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			writeJSON(w, map[string]string{"error": "Failed to read request body."})
			return
		}
		version := strings.TrimSpace(string(bodyBytes))

		state := imagehelper.GetDownload(version)
		if state == nil {
			writeJSON(w, map[string]string{"error": "Download state is unrecognised."})
			return
		}

		filename, progress, done, sErr := state.GetStatus()
		if sErr != nil {
			writeJSON(w, map[string]string{"error": sErr.Error()})
			return
		}
		if done {
			writeJSON(w, map[string]bool{"done": true})
			return
		}

		writeJSON(w, map[string]interface{}{
			"filename": filename,
			"progress": progress,
		})
	})

	mux.HandleFunc("/set_location", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			UDID string  `json:"udid"`
			Lat  float64 `json:"lat"`
			Lng  float64 `json:"lng"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, map[string]string{"error": "Invalid request body."})
			return
		}

		device := s.findDevice(req.UDID)
		if device == nil {
			writeJSON(w, map[string]string{
				"error": "Unable to find the specified device. Are you sure it is connected?",
			})
			return
		}

		if err := location.SetLocation(req.UDID, req.Lat, req.Lng, device); err != nil {
			writeJSON(w, map[string]string{"error": err.Error()})
			return
		}

		writeJSON(w, map[string]bool{"success": true})
	})

	mux.HandleFunc("/stop_location", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			UDID string `json:"udid"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, map[string]string{"error": "Invalid request body."})
			return
		}

		device := s.findDevice(req.UDID)
		if device == nil {
			writeJSON(w, map[string]string{
				"error": "Unable to find the specified device. Are you sure it is connected?",
			})
			return
		}

		if err := location.StopLocation(req.UDID, device); err != nil {
			writeJSON(w, map[string]string{"error": err.Error()})
			return
		}

		writeJSON(w, map[string]bool{"success": true})
	})

	mux.HandleFunc("/exit", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		go func() {
			time.Sleep(100 * time.Millisecond)
			location.CloseAllSessions()
			os.Exit(0)
		}()
	})

	// Static file handler
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "/" || path == "" {
			path = "/main.html"
		}
		path = strings.TrimPrefix(path, "/")

		// Check if local file exists in resources or Resources
		var fileData []byte
		var contentType string
		localPaths := []string{
			filepath.Join("resources", path),
			filepath.Join("Resources", path),
		}

		for _, lp := range localPaths {
			if data, err := os.ReadFile(lp); err == nil {
				fileData = data
				break
			}
		}

		// Fallback to embedded FS
		if fileData == nil {
			embedPath := filepath.Join("resources", path)
			if data, err := s.embeddedFS.ReadFile(embedPath); err == nil {
				fileData = data
			}
		}

		if fileData == nil {
			http.NotFound(w, r)
			return
		}

		ext := filepath.Ext(path)
		switch ext {
		case ".html":
			contentType = "text/html; charset=utf-8"
		case ".css":
			contentType = "text/css; charset=utf-8"
		case ".js":
			contentType = "application/javascript; charset=utf-8"
		case ".png":
			contentType = "image/png"
		case ".json":
			contentType = "application/json; charset=utf-8"
		default:
			contentType = mime.TypeByExtension(ext)
			if contentType == "" {
				contentType = "application/octet-stream"
			}
		}

		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(fileData)
	})

	return mux
}

func detectHomeCountry() string {
	lang := os.Getenv("LC_ALL")
	if lang == "" {
		lang = os.Getenv("LANG")
	}
	if lang == "" {
		lang = os.Getenv("LC_CTYPE")
	}

	langUpper := strings.ToUpper(lang)
	if strings.Contains(langUpper, "CN") || strings.Contains(langUpper, "ZH") {
		return "China"
	}
	if strings.Contains(langUpper, "US") {
		return "United States"
	}
	if strings.Contains(langUpper, "GB") || strings.Contains(langUpper, "UK") {
		return "United Kingdom"
	}
	if strings.Contains(langUpper, "AU") {
		return "Australia"
	}
	if strings.Contains(langUpper, "CA") {
		return "Canada"
	}
	if strings.Contains(langUpper, "DE") {
		return "Germany"
	}
	if strings.Contains(langUpper, "FR") {
		return "France"
	}
	if strings.Contains(langUpper, "JP") {
		return "Japan"
	}
	if strings.Contains(langUpper, "KR") {
		return "South Korea"
	}
	return "China"
}
