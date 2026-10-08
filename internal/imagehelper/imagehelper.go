package imagehelper

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"ifakelocation/internal/models"
)

const ImageBaseDir = "DeveloperImages"

var (
	MobileImageFileList = []string{
		"DeveloperDiskImage.dmg",
		"DeveloperDiskImage.dmg.signature",
	}

	PersonalisedImageFileList = []string{
		"Image.dmg",
		"BuildManifest.plist",
		"Image.dmg.trustcache",
	}

	VersionMapping = map[string]string{
		"12.4": "12.3",
	}

	httpClient = &http.Client{
		Timeout: 60 * time.Second,
	}

	urlCacheMu             sync.Mutex
	versionToImageUrlOverride = make(map[string]string)
	versionToImageUrlZip      = make(map[string]string)
	versionToImageUrlLegacy   = make(map[string]string)

	downloadsMu sync.Mutex
	downloads   = make(map[string]*DownloadState)
)

type DownloadState struct {
	Links        []string
	Paths        []string
	CurrentIndex int
	Progress     float32
	Error        error
	Done         bool
	mu           sync.Mutex
}

func (s *DownloadState) GetStatus() (filename string, progress float32, done bool, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Error != nil {
		return "", 0, false, s.Error
	}
	if s.Done {
		return "", 100, true, nil
	}
	if s.CurrentIndex < len(s.Paths) {
		return filepath.Base(s.Paths[s.CurrentIndex]), s.Progress, false, nil
	}
	return "", s.Progress, false, nil
}

func GetSoftwareVersion(productVersion string) string {
	parts := strings.Split(productVersion, ".")
	if len(parts) == 0 || parts[0] == "" {
		return "16.0"
	}
	v := parts[0]
	if len(parts) > 1 {
		v = parts[0] + "." + parts[1]
	}
	if mapped, ok := VersionMapping[v]; ok {
		return mapped
	}
	return v
}

func IsKnownImageFileName(fileName string) bool {
	base := filepath.Base(fileName)
	for _, f := range MobileImageFileList {
		if strings.EqualFold(base, f) {
			return true
		}
	}
	for _, f := range PersonalisedImageFileList {
		if strings.EqualFold(base, f) {
			return true
		}
	}
	return false
}

func checkFilesExist(files []string) bool {
	for _, f := range files {
		info, err := os.Stat(f)
		if err != nil || info.IsDir() {
			return false
		}
	}
	return true
}

func isPersonalizedDDIValid(dir string) (bool, []string) {
	manifest := filepath.Join(dir, "BuildManifest.plist")
	if _, err := os.Stat(manifest); err != nil {
		return false, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, nil
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".dmg") {
			return true, []string{filepath.Join(dir, e.Name()), manifest}
		}
	}
	return false, nil
}

// HasImageForDevice checks if Developer Images exist for device
func HasImageForDevice(device *models.DeviceInformation) (bool, []string, string) {
	verStr := GetSoftwareVersion(device.ProductVersion)
	majorVer := 16
	parts := strings.Split(verStr, ".")
	if m, err := strconv.Atoi(parts[0]); err == nil {
		majorVer = m
	}

	if majorVer >= 17 {
		candidateDirs := []string{
			filepath.Join(ImageBaseDir, verStr),
			filepath.Join(ImageBaseDir, "17.0"),
			filepath.Join(ImageBaseDir, "Personalized"),
			filepath.Join(ImageBaseDir, "ddi-17E5179g", "Restore"),
			filepath.Join(ImageBaseDir, "ddi-17E5179g"),
			"/Library/Developer/DeveloperDiskImages/iOS_DDI/Restore",
		}

		searchBases := []string{
			".",
			filepath.Join(".", ".."),
			filepath.Join(".", "..", ".."),
		}

		for _, base := range searchBases {
			for _, cDir := range candidateDirs {
				target := cDir
				if !filepath.IsAbs(cDir) {
					target = filepath.Join(base, cDir)
				}
				if ok, paths := isPersonalizedDDIValid(target); ok {
					return true, paths, verStr
				}
			}

			// Also check any folder inside DeveloperImages
			devImagesDir := filepath.Join(base, ImageBaseDir)
			if entries, err := os.ReadDir(devImagesDir); err == nil {
				for _, entry := range entries {
					if entry.IsDir() {
						if ok, paths := isPersonalizedDDIValid(filepath.Join(devImagesDir, entry.Name())); ok {
							return true, paths, verStr
						}
						if ok, paths := isPersonalizedDDIValid(filepath.Join(devImagesDir, entry.Name(), "Restore")); ok {
							return true, paths, verStr
						}
					}
				}
			}
		}

		return false, nil, verStr
	}

	expectedNames := MobileImageFileList
	candidateDirs := []string{verStr}

	searchBases := []string{
		".",
		filepath.Join(".", ".."),
		filepath.Join(".", "..", ".."),
	}

	for _, base := range searchBases {
		for _, cDir := range candidateDirs {
			var fullPaths []string
			for _, name := range expectedNames {
				fullPaths = append(fullPaths, filepath.Join(base, ImageBaseDir, cDir, name))
			}
			if checkFilesExist(fullPaths) {
				return true, fullPaths, verStr
			}
		}
	}

	return false, nil, verStr
}

type DownloadPair struct {
	URL  string
	Path string
}

// GetLinksForDevice resolves download links for a device
func GetLinksForDevice(device *models.DeviceInformation) []DownloadPair {
	verStr := GetSoftwareVersion(device.ProductVersion)
	majorVer := 16
	parts := strings.Split(verStr, ".")
	if m, err := strconv.Atoi(parts[0]); err == nil {
		majorVer = m
	}

	urlCacheMu.Lock()
	defer urlCacheMu.Unlock()

	// 1. Load local updates.json or fetch remote updates.json
	if len(versionToImageUrlOverride) == 0 {
		loadUpdatesJson()
	}

	// 2. Load zip repository images
	if len(versionToImageUrlZip) == 0 {
		loadHaikieuZipImages()
	}

	// 3. Load legacy repository images
	if len(versionToImageUrlLegacy) == 0 {
		loadXushuduoLegacyImages()
	}

	targetURL := ""
	if u, ok := versionToImageUrlOverride[verStr]; ok {
		targetURL = u
	} else if u, ok := versionToImageUrlZip[verStr]; ok {
		targetURL = u
	} else if u, ok := versionToImageUrlLegacy[verStr]; ok {
		targetURL = u
	}

	// For iOS 17+, use the Universal Personalized Developer Disk Image if no specific URL found
	if targetURL == "" && majorVer >= 17 {
		if u, ok := versionToImageUrlOverride["17.0"]; ok {
			targetURL = u
		} else {
			targetURL = "https://deviceboxhq.com/ddi-17E5179g.zip"
		}
	}

	// For iOS < 17, fallback to highest available matching version
	if targetURL == "" && majorVer < 17 {
		var bestVer string
		for v := range versionToImageUrlOverride {
			if strings.HasPrefix(v, fmt.Sprintf("%d.", majorVer)) {
				if bestVer == "" || v > bestVer {
					bestVer = v
				}
			}
		}
		if bestVer != "" {
			targetURL = versionToImageUrlOverride[bestVer]
		}
		if targetURL == "" {
			for v := range versionToImageUrlZip {
				if strings.HasPrefix(v, fmt.Sprintf("%d.", majorVer)) {
					if bestVer == "" || v > bestVer {
						bestVer = v
					}
				}
			}
			if bestVer != "" {
				targetURL = versionToImageUrlZip[bestVer]
			}
		}
	}

	if targetURL == "" {
		return nil
	}

	destDir := filepath.Join(ImageBaseDir, verStr)
	if strings.HasSuffix(strings.ToLower(targetURL), ".zip") {
		return []DownloadPair{
			{URL: targetURL, Path: filepath.Join(destDir, verStr+".zip")},
		}
	}

	return []DownloadPair{
		{URL: targetURL, Path: filepath.Join(destDir, "DeveloperDiskImage.dmg")},
		{URL: targetURL + ".signature", Path: filepath.Join(destDir, "DeveloperDiskImage.dmg.signature")},
	}
}

func loadUpdatesJson() {
	// Try local updates.json first
	data, err := os.ReadFile("updates.json")
	if err != nil {
		req, _ := http.NewRequest("GET", "https://raw.githubusercontent.com/master131/iFakeLocation/master/updates.json", nil)
		req.Header.Set("User-Agent", "iFakeLocation-Go")
		resp, err := httpClient.Do(req)
		if err == nil && resp.StatusCode == 200 {
			data, _ = io.ReadAll(resp.Body)
			resp.Body.Close()
		}
	}

	if len(data) > 0 {
		var parsed struct {
			Images map[string]string `json:"images"`
		}
		if err := json.Unmarshal(data, &parsed); err == nil {
			for k, v := range parsed.Images {
				versionToImageUrlOverride[k] = v
			}
		}
	}
}

func loadHaikieuZipImages() {
	req, _ := http.NewRequest("GET", "https://github.com/haikieu/xcode-developer-disk-image-all-platforms/find/master?_pjax=%23js-repo-pjax-container", nil)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("User-Agent", "iFakeLocation-Go")
	resp, err := httpClient.Do(req)
	treeList := "89cdf804bd416d0d6ba3f958b5c6d086cb914fa1"
	if err == nil && resp.StatusCode == 200 {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		s := string(b)
		tl := "/tree-list/"
		idx := strings.Index(s, tl)
		if idx != -1 {
			end := strings.Index(s[idx+len(tl):], "\"")
			if end != -1 {
				treeList = s[idx+len(tl) : idx+len(tl)+end]
			}
		}
	}

	req2, _ := http.NewRequest("GET", "https://github.com/haikieu/xcode-developer-disk-image-all-platforms/tree-list/"+treeList, nil)
	req2.Header.Set("Accept", "application/json")
	req2.Header.Set("X-Requested-With", "XMLHttpRequest")
	req2.Header.Set("User-Agent", "iFakeLocation-Go")
	resp2, err := httpClient.Do(req2)
	if err == nil && resp2.StatusCode == 200 {
		b2, _ := io.ReadAll(resp2.Body)
		resp2.Body.Close()
		for _, part := range strings.Split(string(b2), "\"") {
			if strings.HasSuffix(strings.ToLower(part), ".zip") && strings.Contains(part, "iPhoneOS") {
				base := filepath.Base(part)
				ver := strings.TrimSuffix(base, filepath.Ext(base))
				versionToImageUrlZip[ver] = "https://github.com/haikieu/xcode-developer-disk-image-all-platforms/raw/master/" + part
			}
		}
	}
}

func loadXushuduoLegacyImages() {
	req, _ := http.NewRequest("GET", "https://github.com/xushuduo/Xcode-iOS-Developer-Disk-Image/find/master?_pjax=%23js-repo-pjax-container", nil)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("User-Agent", "iFakeLocation-Go")
	resp, err := httpClient.Do(req)
	treeList := "795fc91f28cb3884edc45b876482911c797de85c"
	if err == nil && resp.StatusCode == 200 {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		s := string(b)
		tl := "/tree-list/"
		idx := strings.Index(s, tl)
		if idx != -1 {
			end := strings.Index(s[idx+len(tl):], "\"")
			if end != -1 {
				treeList = s[idx+len(tl) : idx+len(tl)+end]
			}
		}
	}

	req2, _ := http.NewRequest("GET", "https://github.com/xushuduo/Xcode-iOS-Developer-Disk-Image/tree-list/"+treeList, nil)
	req2.Header.Set("Accept", "application/json")
	req2.Header.Set("X-Requested-With", "XMLHttpRequest")
	req2.Header.Set("User-Agent", "iFakeLocation-Go")
	resp2, err := httpClient.Do(req2)
	if err == nil && resp2.StatusCode == 200 {
		b2, _ := io.ReadAll(resp2.Body)
		resp2.Body.Close()
		for _, part := range strings.Split(string(b2), "\"") {
			if strings.HasSuffix(strings.ToLower(part), ".dmg") {
				segments := strings.Split(part, "/")
				if len(segments) >= 2 {
					ver := strings.Split(segments[1], " ")[0]
					versionToImageUrlLegacy[ver] = "https://github.com/xushuduo/Xcode-iOS-Developer-Disk-Image/raw/master/" + part
				}
			}
		}
	}
}

// StartDownload starts downloading files for version in background
func StartDownload(version string, pairs []DownloadPair) *DownloadState {
	downloadsMu.Lock()
	defer downloadsMu.Unlock()

	if existing, ok := downloads[version]; ok {
		existing.mu.Lock()
		if !existing.Done && existing.Error == nil {
			existing.mu.Unlock()
			return existing
		}
		existing.mu.Unlock()
	}

	links := make([]string, len(pairs))
	paths := make([]string, len(pairs))
	for i, p := range pairs {
		links[i] = p.URL
		paths[i] = p.Path
	}

	state := &DownloadState{
		Links: links,
		Paths: paths,
	}
	downloads[version] = state

	go state.run()
	return state
}

func GetDownload(version string) *DownloadState {
	downloadsMu.Lock()
	defer downloadsMu.Unlock()
	return downloads[version]
}

func (s *DownloadState) run() {
	for i := 0; i < len(s.Links); i++ {
		s.mu.Lock()
		s.CurrentIndex = i
		s.Progress = 0
		s.mu.Unlock()

		link := s.Links[i]
		destPath := s.Paths[i]
		incompletePath := destPath + ".incomplete"

		dir := filepath.Dir(destPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			s.mu.Lock()
			s.Error = err
			s.mu.Unlock()
			return
		}

		if err := s.downloadFile(link, incompletePath); err != nil {
			s.mu.Lock()
			s.Error = err
			s.mu.Unlock()
			return
		}

		_ = os.Remove(destPath)
		if err := os.Rename(incompletePath, destPath); err != nil {
			s.mu.Lock()
			s.Error = err
			s.mu.Unlock()
			return
		}

		// If zip file, extract known files
		if strings.HasSuffix(strings.ToLower(destPath), ".zip") {
			if err := extractZipFile(destPath, dir); err != nil {
				s.mu.Lock()
				s.Error = err
				s.mu.Unlock()
				return
			}
			_ = os.Remove(destPath)
		}
	}

	s.mu.Lock()
	s.Done = true
	s.mu.Unlock()
}

func (s *DownloadState) downloadFile(url, dest string) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "iFakeLocation-Go")

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP error %d", resp.StatusCode)
	}

	contentLength := resp.ContentLength
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	buf := make([]byte, 32*1024)
	var totalRead int64

	for {
		nr, rerr := resp.Body.Read(buf)
		if nr > 0 {
			nw, werr := out.Write(buf[0:nr])
			if werr != nil {
				return werr
			}
			totalRead += int64(nw)
			if contentLength > 0 {
				s.mu.Lock()
				s.Progress = float32(totalRead) / float32(contentLength) * 100.0
				s.mu.Unlock()
			}
		}
		if rerr != nil {
			if rerr == io.EOF {
				break
			}
			return rerr
		}
	}

	return nil
}

func extractZipFile(zipPath, destDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		baseName := filepath.Base(f.Name)
		if !IsKnownImageFileName(baseName) {
			continue
		}

		targetFile := filepath.Join(destDir, baseName)
		rc, err := f.Open()
		if err != nil {
			return err
		}

		outFile, err := os.OpenFile(targetFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			rc.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()
		if err != nil {
			return err
		}
	}

	return nil
}
