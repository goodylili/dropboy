package server

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/goodylili/dropboy/internal/config"
)

// ---- wire types (mirrored in frontend/src/lib/types.ts) ----

type Machine struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Online bool   `json:"online"`
}

type FileNode struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Kind      string     `json:"kind"`
	Size      int64      `json:"size"`
	Modified  string     `json:"modified"`
	Machine   string     `json:"machine"`
	Path      string     `json:"path"`
	Status    string     `json:"status"`
	Encrypted bool       `json:"encrypted"`
	Tags      []string   `json:"tags,omitempty"`
	Children  []FileNode `json:"children,omitempty"`
}

type Status struct {
	Running        bool   `json:"running"`
	Locked         bool   `json:"locked"`
	Paused         bool   `json:"paused"`
	QueueUploads   int    `json:"queueUploads"`
	QueueDownloads int    `json:"queueDownloads"`
	BytesUp        int64  `json:"bytesUp"`
	BytesDown      int64  `json:"bytesDown"`
	Conflicts      int    `json:"conflicts"`
	LastSyncAt     string `json:"lastSyncAt"`
	Bucket         string `json:"bucket"`
	Region         string `json:"region"`
	MachineID      string `json:"machineId"`
}

type Conflict struct {
	ID       string `json:"id"`
	Path     string `json:"path"`
	Machine  string `json:"machine"`
	Detected string `json:"detected"`
	Local    string `json:"local"`
	Remote   string `json:"remote"`
}

// ---- handlers ----

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleMachines(w http.ResponseWriter, _ *http.Request) {
	s.mu.RLock()
	cfg := s.Cfg
	s.mu.RUnlock()
	id := cfg.MachineID
	if id == "" {
		if h, err := os.Hostname(); err == nil {
			id = h
		} else {
			id = "this-machine"
		}
	}
	ms := []Machine{{ID: id, Label: id, Online: true}}
	writeJSON(w, http.StatusOK, ms)
}

func (s *Server) handleTree(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	machine := q.Get("machine")
	path := q.Get("path")
	if path == "" {
		path = "/"
	}

	s.mu.RLock()
	cfg := s.Cfg
	s.mu.RUnlock()

	localID := cfg.MachineID
	if localID == "" {
		if h, err := os.Hostname(); err == nil {
			localID = h
		}
	}

	if machine != "" && machine != localID {
		writeJSON(w, http.StatusOK, emptyRoot(machine))
		return
	}

	root := s.buildLocalTree(cfg, localID)
	node := findByPath(&root, path)
	if node == nil {
		writeJSONError(w, http.StatusNotFound, "path not found")
		return
	}
	writeJSON(w, http.StatusOK, *node)
}

func emptyRoot(machine string) FileNode {
	return FileNode{
		ID:       "root-" + machine,
		Name:     "/",
		Kind:     "folder",
		Path:     "/",
		Machine:  machine,
		Modified: time.Now().UTC().Format(time.RFC3339),
	}
}

// buildLocalTree synthesizes a virtual root containing one child per watched
// folder. Each folder is walked once with a bounded depth and entry count so
// the API stays responsive on large trees.
func (s *Server) buildLocalTree(cfg config.Config, machine string) FileNode {
	root := emptyRoot(machine)
	root.Machine = machine
	for _, f := range cfg.Folders {
		node, err := walkDir(f.Path, machine, f.Exclude)
		if err != nil {
			continue
		}
		root.Children = append(root.Children, node)
		root.Size += node.Size
		if node.Modified > root.Modified {
			root.Modified = node.Modified
		}
	}
	return root
}

const (
	maxChildrenPerDir = 500
	maxWalkDepth      = 8
)

func walkDir(absPath, machine string, excludes []string) (FileNode, error) {
	info, err := os.Stat(absPath)
	if err != nil {
		return FileNode{}, err
	}
	if !info.IsDir() {
		return FileNode{}, errors.New("not a directory")
	}
	return buildNode(absPath, absPath, machine, excludes, 0)
}

func buildNode(root, abs, machine string, excludes []string, depth int) (FileNode, error) {
	info, err := os.Stat(abs)
	if err != nil {
		return FileNode{}, err
	}
	rel, _ := filepath.Rel(filepath.Dir(root), abs)
	virtual := "/" + filepath.ToSlash(rel)
	node := FileNode{
		ID:        hashID(machine + ":" + abs),
		Name:      info.Name(),
		Modified:  info.ModTime().UTC().Format(time.RFC3339),
		Machine:   machine,
		Path:      virtual,
		Status:    "synced",
		Encrypted: true,
	}
	if info.IsDir() {
		node.Kind = "folder"
		if depth >= maxWalkDepth {
			return node, nil
		}
		entries, err := os.ReadDir(abs)
		if err != nil {
			return node, nil
		}
		sort.Slice(entries, func(i, j int) bool {
			if entries[i].IsDir() != entries[j].IsDir() {
				return entries[i].IsDir()
			}
			return entries[i].Name() < entries[j].Name()
		})
		count := 0
		for _, e := range entries {
			if count >= maxChildrenPerDir {
				break
			}
			if excluded(e.Name(), excludes) {
				continue
			}
			child, err := buildNode(root, filepath.Join(abs, e.Name()), machine, excludes, depth+1)
			if err != nil {
				continue
			}
			node.Children = append(node.Children, child)
			node.Size += child.Size
			count++
		}
		return node, nil
	}
	node.Kind = kindFor(info.Name())
	node.Size = info.Size()
	return node, nil
}

func excluded(name string, patterns []string) bool {
	for _, p := range patterns {
		ok, _ := filepath.Match(p, name)
		if ok {
			return true
		}
	}
	return false
}

func hashID(s string) string {
	sum := sha1.Sum([]byte(s))
	return hex.EncodeToString(sum[:8])
}

func kindFor(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".heic", ".svg", ".bmp", ".tiff":
		return "image"
	case ".mp4", ".mov", ".mkv", ".webm", ".avi":
		return "video"
	case ".mp3", ".m4a", ".wav", ".flac", ".ogg", ".aac":
		return "audio"
	case ".pdf":
		return "pdf"
	case ".go", ".ts", ".tsx", ".js", ".jsx", ".py", ".rs", ".c", ".h", ".cpp", ".java", ".rb", ".sh", ".yaml", ".yml", ".toml", ".json", ".mod":
		return "code"
	case ".zip", ".tar", ".gz", ".bz2", ".7z", ".rar":
		return "archive"
	case ".doc", ".docx", ".odt", ".rtf":
		return "document"
	case ".xls", ".xlsx", ".csv", ".ods":
		return "spreadsheet"
	case ".ppt", ".pptx", ".key", ".odp":
		return "presentation"
	case ".txt", ".md", ".log":
		return "text"
	}
	if strings.EqualFold(name, "makefile") || strings.EqualFold(name, "dockerfile") {
		return "code"
	}
	return "binary"
}

func findByPath(root *FileNode, path string) *FileNode {
	if path == "" || path == "/" {
		return root
	}
	segs := strings.Split(strings.Trim(path, "/"), "/")
	cur := root
	for _, seg := range segs {
		var next *FileNode
		for i := range cur.Children {
			if cur.Children[i].Name == seg {
				next = &cur.Children[i]
				break
			}
		}
		if next == nil {
			return nil
		}
		cur = next
	}
	return cur
}

// ---- status / settings ----

func (s *Server) handleStatus(w http.ResponseWriter, _ *http.Request) {
	s.mu.RLock()
	cfg := s.Cfg
	eng := s.engine
	s.mu.RUnlock()
	st := Status{
		Running:   true,
		Bucket:    cfg.Bucket,
		Region:    cfg.Region,
		MachineID: cfg.MachineID,
	}
	if eng != nil {
		es := eng.Stats()
		st.Locked = eng.Locked()
		st.Paused = es.Paused
		st.QueueUploads = es.QueueUp
		st.QueueDownloads = es.QueueDown
		st.BytesUp = es.BytesUp
		st.BytesDown = es.BytesDown
		st.Conflicts = es.Conflicts
		if !es.LastRun.IsZero() {
			st.LastSyncAt = es.LastRun.Format(time.RFC3339)
		}
	}
	if st.LastSyncAt == "" {
		st.LastSyncAt = time.Now().UTC().Format(time.RFC3339)
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) handleSettingsGet(w http.ResponseWriter, _ *http.Request) {
	s.mu.RLock()
	cfg := s.Cfg
	s.mu.RUnlock()
	writeJSON(w, http.StatusOK, cfg)
}

func (s *Server) handleSettingsPut(w http.ResponseWriter, r *http.Request) {
	var incoming config.Config
	if err := readJSON(r, &incoming); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.mu.Lock()
	// Preserve immutable bits the UI must not change here.
	incoming.Folders = s.Cfg.Folders
	s.Cfg = incoming
	s.mu.Unlock()
	if err := config.Save(incoming); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, incoming)
}

// ---- folders ----

type folderPayload struct {
	Path    string   `json:"path"`
	Exclude []string `json:"exclude"`
}

func (s *Server) handleFoldersList(w http.ResponseWriter, _ *http.Request) {
	s.mu.RLock()
	folders := append([]config.Folder(nil), s.Cfg.Folders...)
	s.mu.RUnlock()
	writeJSON(w, http.StatusOK, folders)
}

func (s *Server) handleFoldersAdd(w http.ResponseWriter, r *http.Request) {
	var p folderPayload
	if err := readJSON(r, &p); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if p.Path == "" {
		writeJSONError(w, http.StatusBadRequest, "path required")
		return
	}
	abs, err := filepath.Abs(p.Path)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	st, err := os.Stat(abs)
	if err != nil || !st.IsDir() {
		writeJSONError(w, http.StatusBadRequest, "path is not an accessible directory")
		return
	}
	s.mu.Lock()
	for _, f := range s.Cfg.Folders {
		if f.Path == abs {
			s.mu.Unlock()
			writeJSONError(w, http.StatusConflict, "already watched")
			return
		}
	}
	s.Cfg.Folders = append(s.Cfg.Folders, config.Folder{Path: abs, Exclude: p.Exclude})
	cfg := s.Cfg
	s.mu.Unlock()
	if err := config.Save(cfg); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, cfg.Folders)
}

func (s *Server) handleFoldersRemove(w http.ResponseWriter, r *http.Request) {
	target := r.URL.Query().Get("path")
	if target == "" {
		writeJSONError(w, http.StatusBadRequest, "path required")
		return
	}
	abs, err := filepath.Abs(target)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.mu.Lock()
	next := s.Cfg.Folders[:0]
	removed := false
	for _, f := range s.Cfg.Folders {
		if f.Path == abs {
			removed = true
			continue
		}
		next = append(next, f)
	}
	s.Cfg.Folders = next
	cfg := s.Cfg
	s.mu.Unlock()
	if !removed {
		writeJSONError(w, http.StatusNotFound, "not watched")
		return
	}
	if err := config.Save(cfg); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, cfg.Folders)
}

// ---- conflicts (stubs until sync engine lands) ----

func (s *Server) handleConflictsList(w http.ResponseWriter, _ *http.Request) {
	s.mu.RLock()
	eng := s.engine
	s.mu.RUnlock()
	if eng == nil {
		writeJSON(w, http.StatusOK, []Conflict{})
		return
	}
	writeJSON(w, http.StatusOK, eng.Conflicts())
}

func (s *Server) handleConflictsResolve(w http.ResponseWriter, r *http.Request) {
	var p struct {
		ID         string `json:"id"`
		Resolution string `json:"resolution"`
	}
	if err := readJSON(r, &p); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.mu.RLock()
	eng := s.engine
	s.mu.RUnlock()
	resolved := false
	if eng != nil {
		resolved = eng.ResolveConflict(p.ID)
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": p.ID, "resolution": p.Resolution, "resolved": resolved})
}

// ---- restore / sync / pause (stubs) ----

func (s *Server) handleRestore(w http.ResponseWriter, r *http.Request) {
	var p struct {
		Machine string `json:"machine"`
		Path    string `json:"path"`
		Into    string `json:"into"`
	}
	if err := readJSON(r, &p); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if p.Machine == "" {
		writeJSONError(w, http.StatusBadRequest, "machine required")
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"status": "queued", "machine": p.Machine, "path": p.Path, "into": p.Into})
}

func (s *Server) handleSync(w http.ResponseWriter, _ *http.Request) {
	s.mu.RLock()
	eng := s.engine
	s.mu.RUnlock()
	s.events.publish(Event{Type: "sync.kick", Time: time.Now().UTC()})
	if eng != nil {
		go func() { _ = eng.KickSync() }()
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "queued"})
}

func (s *Server) handlePause(w http.ResponseWriter, _ *http.Request) {
	s.mu.RLock()
	eng := s.engine
	s.mu.RUnlock()
	if eng != nil {
		eng.SetPaused(true)
	}
	s.events.publish(Event{Type: "sync.paused", Time: time.Now().UTC()})
	writeJSON(w, http.StatusOK, map[string]bool{"paused": true})
}

func (s *Server) handleResume(w http.ResponseWriter, _ *http.Request) {
	s.mu.RLock()
	eng := s.engine
	s.mu.RUnlock()
	if eng != nil {
		eng.SetPaused(false)
	}
	s.events.publish(Event{Type: "sync.resumed", Time: time.Now().UTC()})
	writeJSON(w, http.StatusOK, map[string]bool{"paused": false})
}

// ---- unlock / keychain ----

func (s *Server) handleUnlock(w http.ResponseWriter, r *http.Request) {
	var p struct {
		Passphrase string `json:"passphrase"`
		Remember   bool   `json:"remember"`
	}
	if err := readJSON(r, &p); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if p.Passphrase == "" {
		writeJSONError(w, http.StatusBadRequest, "passphrase required")
		return
	}
	s.mu.RLock()
	eng := s.engine
	s.mu.RUnlock()
	if eng == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "daemon not ready")
		return
	}
	if err := eng.Unlock(p.Passphrase, p.Remember); err != nil {
		writeJSONError(w, http.StatusUnauthorized, err.Error())
		return
	}
	s.events.publish(Event{Type: "unlocked", Time: time.Now().UTC()})
	writeJSON(w, http.StatusOK, map[string]bool{"unlocked": true, "remembered": p.Remember})
}

func (s *Server) handleForgetPassphrase(w http.ResponseWriter, _ *http.Request) {
	s.mu.RLock()
	eng := s.engine
	s.mu.RUnlock()
	if eng == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "daemon not ready")
		return
	}
	if err := eng.ForgetPassphrase(); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"forgotten": true})
}

// ---- file content (preview / download) ----

func (s *Server) handleFileGet(w http.ResponseWriter, r *http.Request) {
	machine := r.URL.Query().Get("machine")
	path := r.URL.Query().Get("path")
	if path == "" {
		writeJSONError(w, http.StatusBadRequest, "path required")
		return
	}

	s.mu.RLock()
	cfg := s.Cfg
	s.mu.RUnlock()
	localID := cfg.MachineID
	if localID == "" {
		localID, _ = os.Hostname()
	}
	if machine != "" && machine != localID {
		writeJSONError(w, http.StatusNotImplemented, "remote-machine streaming requires the sync engine (pending)")
		return
	}

	abs := resolveWatched(cfg.Folders, path)
	if abs == "" {
		writeJSONError(w, http.StatusNotFound, "path is not under a watched folder")
		return
	}
	info, err := os.Stat(abs)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, err.Error())
		return
	}
	if info.IsDir() {
		writeJSONError(w, http.StatusBadRequest, "path is a directory")
		return
	}
	f, err := os.Open(abs)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer f.Close()
	http.ServeContent(w, r, info.Name(), info.ModTime(), f)
	_ = io.EOF // keep import
}

// resolveWatched maps a virtual /<folder-name>/<rest> path to an absolute
// filesystem path, but only when the resolved path lies under one of the
// configured folders (no escape via ..).
func resolveWatched(folders []config.Folder, virtual string) string {
	segs := strings.Split(strings.Trim(virtual, "/"), "/")
	if len(segs) == 0 || segs[0] == "" {
		return ""
	}
	for _, f := range folders {
		if filepath.Base(f.Path) != segs[0] {
			continue
		}
		joined := filepath.Join(append([]string{f.Path}, segs[1:]...)...)
		clean, err := filepath.Abs(joined)
		if err != nil {
			continue
		}
		base, err := filepath.Abs(f.Path)
		if err != nil {
			continue
		}
		if clean == base || strings.HasPrefix(clean, base+string(filepath.Separator)) {
			return clean
		}
	}
	return ""
}
