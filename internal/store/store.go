// Package store keeps save backups as zip archives.
//
// Layout: <root>/<game id>/<timestamp>-<kind>.zip. Each archive holds the save
// folder under "save/" and a manifest.json with the backup's name, kind and a
// SHA-256 of every file, so a restore can prove it wrote back exactly what was saved.
package store

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	manifestName  = "manifest.json"
	dataPrefix    = "save/"
	formatVersion = 1
)

var ErrEmptySource = errors.New("the save folder is empty")

type Kind string

const (
	Manual Kind = "manual" // named by the user
	Quick  Kind = "quick"  // the single quick-save slot
	Auto   Kind = "auto"   // safety copy taken before a restore
)

type File struct {
	Path    string    `json:"path"` // slash-separated, relative to the save folder
	Size    int64     `json:"size"`
	SHA256  string    `json:"sha256,omitempty"`
	ModTime time.Time `json:"modTime"`
}

type Manifest struct {
	Format  int       `json:"format"`
	Game    string    `json:"game"`
	Name    string    `json:"name"`
	Kind    Kind      `json:"kind"`
	Created time.Time `json:"created"`
	Source  string    `json:"source"`
	Files   []File    `json:"files"`
}

func (m Manifest) TotalSize() int64 {
	var n int64
	for _, f := range m.Files {
		n += f.Size
	}
	return n
}

// Matches reports whether files (hashed by Scan) are byte-for-byte what this backup holds.
func (m Manifest) Matches(files []File) bool {
	if len(files) != len(m.Files) {
		return false
	}
	want := make(map[string]File, len(m.Files))
	for _, f := range m.Files {
		want[f.Path] = f
	}
	for _, f := range files {
		w, ok := want[f.Path]
		if !ok || w.Size != f.Size || w.SHA256 != f.SHA256 {
			return false
		}
	}
	return true
}

type Backup struct {
	Manifest
	Path    string // the archive on disk
	ZipSize int64
	// Err is set when the archive can't be read; such backups are listed but can't be restored.
	Err error
}

type Store struct{ root string }

func New(root string) *Store { return &Store{root: root} }

func (s *Store) Root() string { return s.root }

func (s *Store) GameDir(game string) string { return filepath.Join(s.root, game) }

// Scan lists the regular files under dir, sorted by path. With hash set it also computes their SHA-256.
func Scan(dir string, hash bool) ([]File, error) {
	var files []File
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.Type().IsRegular() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		f := File{Path: filepath.ToSlash(rel), Size: info.Size(), ModTime: info.ModTime()}
		if hash {
			if f.SHA256, err = hashFile(p); err != nil {
				return err
			}
		}
		files = append(files, f)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, nil
}

// Create archives the save folder src. The archive only appears under its final
// name once it is completely written, so a crash never leaves a half-written backup.
func (s *Store) Create(game, src, name string, kind Kind) (_ *Backup, err error) {
	files, err := Scan(src, false)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, ErrEmptySource
	}

	dir := s.GameDir(game)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*.zip")
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			tmp.Close()
			os.Remove(tmp.Name())
		}
	}()

	m := Manifest{Format: formatVersion, Game: game, Name: name, Kind: kind, Created: time.Now(), Source: src}
	zw := zip.NewWriter(tmp)
	for _, f := range files {
		if err := addFile(zw, src, &f); err != nil {
			return nil, err
		}
		m.Files = append(m.Files, f)
	}
	if err := writeManifest(zw, m); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	if err := tmp.Sync(); err != nil {
		return nil, err
	}
	if err := tmp.Close(); err != nil {
		return nil, err
	}

	final := uniquePath(dir, m.Created.Format("20060102-150405")+"-"+string(kind), ".zip")
	if err := os.Rename(tmp.Name(), final); err != nil {
		return nil, err
	}
	return Open(final)
}

func addFile(zw *zip.Writer, root string, f *File) error {
	in, err := os.Open(filepath.Join(root, filepath.FromSlash(f.Path)))
	if err != nil {
		return err
	}
	defer in.Close()

	w, err := zw.CreateHeader(&zip.FileHeader{Name: dataPrefix + f.Path, Method: zip.Deflate, Modified: f.ModTime})
	if err != nil {
		return err
	}
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(w, h), in)
	if err != nil {
		return fmt.Errorf("reading %s: %w", f.Path, err)
	}
	f.Size = n
	f.SHA256 = hex.EncodeToString(h.Sum(nil))
	return nil
}

func writeManifest(zw *zip.Writer, m Manifest) error {
	w, err := zw.Create(manifestName)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(m)
}

// Open reads a backup's manifest.
func Open(p string) (*Backup, error) {
	st, err := os.Stat(p)
	if err != nil {
		return nil, err
	}
	zr, err := zip.OpenReader(p)
	if err != nil {
		return nil, fmt.Errorf("archive is damaged: %w", err)
	}
	defer zr.Close()

	for _, f := range zr.File {
		if f.Name != manifestName {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("archive is damaged: %w", err)
		}
		defer rc.Close()

		var m Manifest
		if err := json.NewDecoder(io.LimitReader(rc, 16<<20)).Decode(&m); err != nil {
			return nil, fmt.Errorf("manifest is damaged: %w", err)
		}
		if m.Format > formatVersion {
			return nil, fmt.Errorf("made by a newer version of the app (format %d)", m.Format)
		}
		return &Backup{Manifest: m, Path: p, ZipSize: st.Size()}, nil
	}
	return nil, errors.New("not a backup: manifest.json is missing")
}

// List returns a game's backups, newest first. Unreadable archives are included with Err set.
func (s *Store) List(game string) ([]*Backup, error) {
	entries, err := os.ReadDir(s.GameDir(game))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var out []*Backup
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || strings.HasPrefix(name, ".") || !strings.EqualFold(filepath.Ext(name), ".zip") {
			continue
		}
		p := filepath.Join(s.GameDir(game), name)
		b, err := Open(p)
		if err != nil {
			b = &Backup{Path: p, Err: err, Manifest: Manifest{Game: game, Name: name}}
			if info, ierr := e.Info(); ierr == nil {
				b.Created, b.ZipSize = info.ModTime(), info.Size()
			}
		}
		out = append(out, b)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Created.After(out[j].Created) })
	return out, nil
}

// Restore replaces the save folder dst with the backup's contents.
//
// Everything is first extracted next to dst and checked against the manifest's
// hashes; only then is the old folder swapped out. If the folder can't be swapped
// as a whole (Windows refuses while something holds it open), its files are replaced one by one.
func (s *Store) Restore(b *Backup, dst string) error {
	if b.Err != nil {
		return b.Err
	}
	parent := filepath.Dir(dst)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(parent, "."+filepath.Base(dst)+".restore-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)

	if err := extract(b, stage); err != nil {
		return err
	}

	if _, err := os.Stat(dst); errors.Is(err, fs.ErrNotExist) {
		return os.Rename(stage, dst)
	}

	old := filepath.Join(parent, fmt.Sprintf(".%s.old-%d", filepath.Base(dst), time.Now().UnixNano()))
	if err := os.Rename(dst, old); err == nil {
		if err := os.Rename(stage, dst); err != nil {
			if rerr := os.Rename(old, dst); rerr != nil {
				return fmt.Errorf("restore failed and the previous save could not be put back (it is in %s): %w", old, err)
			}
			return err
		}
		os.RemoveAll(old)
		return nil
	}
	return replaceContents(stage, dst, b.Files)
}

func extract(b *Backup, dir string) error {
	zr, err := zip.OpenReader(b.Path)
	if err != nil {
		return fmt.Errorf("archive is damaged: %w", err)
	}
	defer zr.Close()

	want := make(map[string]File, len(b.Files))
	for _, f := range b.Files {
		want[f.Path] = f
	}

	for _, zf := range zr.File {
		rel, ok := strings.CutPrefix(zf.Name, dataPrefix)
		if !ok || zf.FileInfo().IsDir() {
			continue
		}
		if !safeRel(rel) {
			return fmt.Errorf("archive contains an unsafe path %q", zf.Name)
		}
		exp, ok := want[rel]
		if !ok {
			return fmt.Errorf("archive contains %q, which is not in its manifest", rel)
		}
		if err := extractFile(zf, filepath.Join(dir, filepath.FromSlash(rel)), exp); err != nil {
			return err
		}
		delete(want, rel)
	}
	for rel := range want {
		return fmt.Errorf("archive is missing %q", rel)
	}
	return nil
}

func extractFile(zf *zip.File, target string, exp File) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	rc, err := zf.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	out, err := os.Create(target)
	if err != nil {
		return err
	}

	h := sha256.New()
	_, err = io.Copy(io.MultiWriter(out, h), rc)
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return fmt.Errorf("extracting %s: %w", exp.Path, err)
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != exp.SHA256 {
		return fmt.Errorf("%s is corrupted in the archive (checksum mismatch)", exp.Path)
	}
	return os.Chtimes(target, exp.ModTime, exp.ModTime)
}

// replaceContents makes dst hold exactly the staged files without renaming dst itself.
// It is all-or-nothing: current files are moved aside first, and if any of them is
// locked, the ones already moved are put back before returning the error.
func replaceContents(stage, dst string, files []File) error {
	current, err := Scan(dst, false)
	if err != nil {
		return err
	}
	aside, err := os.MkdirTemp(dst, ".old-")
	if err != nil {
		return err
	}
	asideName := filepath.Base(aside)

	var moved []string
	undo := func() {
		for i := len(moved) - 1; i >= 0; i-- {
			rel := filepath.FromSlash(moved[i])
			os.Rename(filepath.Join(aside, rel), filepath.Join(dst, rel))
		}
		os.RemoveAll(aside)
	}
	locked := func(err error) error {
		undo()
		return fmt.Errorf("the save folder is in use (is the game running?): %w", err)
	}

	for _, f := range current {
		if strings.HasPrefix(f.Path, asideName+"/") {
			continue
		}
		rel := filepath.FromSlash(f.Path)
		if err := os.MkdirAll(filepath.Dir(filepath.Join(aside, rel)), 0o755); err != nil {
			return locked(err)
		}
		if err := os.Rename(filepath.Join(dst, rel), filepath.Join(aside, rel)); err != nil {
			return locked(err)
		}
		moved = append(moved, f.Path)
	}

	var placed []string
	for _, f := range files {
		rel := filepath.FromSlash(f.Path)
		target := filepath.Join(dst, rel)
		err := os.MkdirAll(filepath.Dir(target), 0o755)
		if err == nil {
			err = os.Rename(filepath.Join(stage, rel), target)
		}
		if err != nil {
			for _, p := range placed {
				os.Remove(filepath.Join(dst, filepath.FromSlash(p)))
			}
			return locked(err)
		}
		placed = append(placed, f.Path)
	}
	return os.RemoveAll(aside)
}

// Rename changes a backup's display name by rewriting its manifest.
func (s *Store) Rename(b *Backup, name string) (*Backup, error) {
	if b.Err != nil {
		return nil, b.Err
	}
	if err := rewrite(b.Path, func(m *Manifest) { m.Name = name }); err != nil {
		return nil, err
	}
	return Open(b.Path)
}

func rewrite(p string, edit func(*Manifest)) (err error) {
	b, err := Open(p)
	if err != nil {
		return err
	}
	m := b.Manifest
	edit(&m)

	zr, err := zip.OpenReader(p)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(p), ".tmp-*.zip")
	if err != nil {
		zr.Close()
		return err
	}
	defer func() {
		if err != nil {
			tmp.Close()
			os.Remove(tmp.Name())
		}
	}()

	zw := zip.NewWriter(tmp)
	for _, f := range zr.File {
		if f.Name == manifestName {
			continue
		}
		if err := zw.Copy(f); err != nil {
			zr.Close()
			return err
		}
	}
	// Windows can't replace a file that is still open.
	zr.Close()
	if err := writeManifest(zw, m); err != nil {
		return err
	}
	if err := zw.Close(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), p)
}

func (s *Store) Delete(b *Backup) error { return os.Remove(b.Path) }

// Prune deletes the oldest backups of kind beyond the newest keep.
func (s *Store) Prune(game string, kind Kind, keep int) error {
	list, err := s.List(game)
	if err != nil {
		return err
	}
	n := 0
	for _, b := range list {
		if b.Err != nil || b.Kind != kind {
			continue
		}
		if n++; n > keep {
			if err := s.Delete(b); err != nil {
				return err
			}
		}
	}
	return nil
}

func hashFile(p string) (string, error) {
	f, err := os.Open(p)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// safeRel rejects archive paths that would escape the folder they're extracted into.
func safeRel(rel string) bool {
	if rel == "" || strings.ContainsAny(rel, `\:`) || path.IsAbs(rel) {
		return false
	}
	clean := path.Clean(rel)
	return clean == rel && clean != ".." && !strings.HasPrefix(clean, "../")
}

func uniquePath(dir, base, ext string) string {
	p := filepath.Join(dir, base+ext)
	for i := 2; ; i++ {
		if _, err := os.Lstat(p); errors.Is(err, fs.ErrNotExist) {
			return p
		}
		p = filepath.Join(dir, fmt.Sprintf("%s-%d%s", base, i, ext))
	}
}
