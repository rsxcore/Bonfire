package store

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func writeTree(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func readTree(t *testing.T, dir string) map[string]string {
	t.Helper()
	files, err := Scan(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, f := range files {
		data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(f.Path)))
		if err != nil {
			t.Fatal(err)
		}
		out[f.Path] = string(data)
	}
	return out
}

func equalTrees(t *testing.T, got, want map[string]string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d files %v, want %d %v", len(got), got, len(want), want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("file %s = %q, want %q", k, got[k], v)
		}
	}
}

func TestCreateRestoreRoundTrip(t *testing.T) {
	root := t.TempDir()
	save := filepath.Join(root, "game", "EldenRing")
	original := map[string]string{
		"76561198000000000/ER0000.sl2":     "slot data",
		"76561198000000000/ER0000.sl2.bak": "backup slot",
		"GraphicsConfig.xml":               "<cfg/>",
	}
	writeTree(t, save, original)

	s := New(filepath.Join(root, "backups"))
	b, err := s.Create("er", save, "Before Malenia", Manual)
	if err != nil {
		t.Fatal(err)
	}
	if b.Name != "Before Malenia" || b.Kind != Manual || len(b.Files) != 3 {
		t.Fatalf("unexpected manifest: %+v", b.Manifest)
	}

	// Change the save: edit a file, add one, remove one.
	writeTree(t, save, map[string]string{"76561198000000000/ER0000.sl2": "died to Malenia", "junk.tmp": "x"})
	os.Remove(filepath.Join(save, "GraphicsConfig.xml"))

	if err := s.Restore(b, save); err != nil {
		t.Fatal(err)
	}
	equalTrees(t, readTree(t, save), original)

	// No staging or old folders are left next to the save.
	entries, _ := os.ReadDir(filepath.Dir(save))
	if len(entries) != 1 {
		t.Fatalf("leftovers next to save folder: %v", entries)
	}
}

func TestRestoreIntoMissingFolder(t *testing.T) {
	root := t.TempDir()
	save := filepath.Join(root, "Sekiro")
	writeTree(t, save, map[string]string{"S0000.sl2": "data"})
	s := New(filepath.Join(root, "backups"))
	b, err := s.Create("sekiro", save, "x", Manual)
	if err != nil {
		t.Fatal(err)
	}
	os.RemoveAll(save)
	if err := s.Restore(b, save); err != nil {
		t.Fatal(err)
	}
	equalTrees(t, readTree(t, save), map[string]string{"S0000.sl2": "data"})
}

func TestReplaceContentsFallback(t *testing.T) {
	root := t.TempDir()
	stage, dst := filepath.Join(root, "stage"), filepath.Join(root, "dst")
	writeTree(t, stage, map[string]string{"a/1.sl2": "new"})
	writeTree(t, dst, map[string]string{"a/1.sl2": "old", "stale.bak": "gone"})
	if err := replaceContents(stage, dst, []File{{Path: "a/1.sl2"}}); err != nil {
		t.Fatal(err)
	}
	equalTrees(t, readTree(t, dst), map[string]string{"a/1.sl2": "new"})
}

func TestCreateEmptySource(t *testing.T) {
	root := t.TempDir()
	s := New(filepath.Join(root, "backups"))
	if _, err := s.Create("ds3", root+"/nothing-here-is-empty", "x", Manual); err == nil {
		t.Fatal("expected error for missing folder")
	}
	empty := filepath.Join(root, "empty")
	os.Mkdir(empty, 0o755)
	if _, err := s.Create("ds3", empty, "x", Manual); err != ErrEmptySource {
		t.Fatalf("got %v, want ErrEmptySource", err)
	}
}

func TestListPruneRename(t *testing.T) {
	root := t.TempDir()
	save := filepath.Join(root, "save")
	writeTree(t, save, map[string]string{"f": "1"})
	s := New(filepath.Join(root, "backups"))

	for i := 0; i < 4; i++ {
		if _, err := s.Create("ds3", save, "auto", Auto); err != nil {
			t.Fatal(err)
		}
	}
	manual, err := s.Create("ds3", save, "mine", Manual)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Prune("ds3", Auto, 2); err != nil {
		t.Fatal(err)
	}
	list, _ := s.List("ds3")
	if len(list) != 3 {
		t.Fatalf("after prune: %d backups, want 3", len(list))
	}

	renamed, err := s.Rename(manual, "Before Nameless King")
	if err != nil {
		t.Fatal(err)
	}
	if renamed.Name != "Before Nameless King" || len(renamed.Files) != 1 {
		t.Fatalf("rename lost data: %+v", renamed.Manifest)
	}
	// The renamed archive still restores.
	if err := s.Restore(renamed, filepath.Join(root, "restored")); err != nil {
		t.Fatal(err)
	}
}

func TestDamagedArchiveIsListedNotRestored(t *testing.T) {
	root := t.TempDir()
	s := New(root)
	os.MkdirAll(s.GameDir("er"), 0o755)
	os.WriteFile(filepath.Join(s.GameDir("er"), "broken.zip"), []byte("not a zip"), 0o644)

	list, err := s.List("er")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Err == nil {
		t.Fatalf("damaged archive should be listed with an error: %+v", list)
	}
	if err := s.Restore(list[0], filepath.Join(root, "dst")); err == nil {
		t.Fatal("restoring a damaged archive must fail")
	}
}

func TestTamperedFileFailsChecksum(t *testing.T) {
	root := t.TempDir()
	save := filepath.Join(root, "save")
	writeTree(t, save, map[string]string{"slot.sl2": "genuine"})
	s := New(filepath.Join(root, "backups"))
	b, err := s.Create("er", save, "x", Manual)
	if err != nil {
		t.Fatal(err)
	}

	// Rebuild the archive with the same manifest but different file content.
	zr, _ := zip.OpenReader(b.Path)
	evil := filepath.Join(root, "evil.zip")
	out, _ := os.Create(evil)
	zw := zip.NewWriter(out)
	for _, f := range zr.File {
		if f.Name == dataPrefix+"slot.sl2" {
			w, _ := zw.Create(f.Name)
			w.Write([]byte("tampered"))
			continue
		}
		zw.Copy(f)
	}
	zw.Close()
	out.Close()
	zr.Close()

	bad, err := Open(evil)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Restore(bad, save); err == nil {
		t.Fatal("expected checksum error")
	}
	// The real save is untouched.
	equalTrees(t, readTree(t, save), map[string]string{"slot.sl2": "genuine"})
}

func TestSafeRel(t *testing.T) {
	for rel, want := range map[string]bool{
		"ER0000.sl2":      true,
		"7656/ER0000.sl2": true,
		"../evil":         false,
		"a/../../evil":    false,
		"/abs":            false,
		"C:/win":          false,
		`a\b`:             false,
		"":                false,
		"a/./b":           false,
		"..":              false,
	} {
		if got := safeRel(rel); got != want {
			t.Errorf("safeRel(%q) = %v, want %v", rel, got, want)
		}
	}
}

func TestMatches(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{"a": "1", "b": "2"})
	s := New(filepath.Join(t.TempDir(), "b"))
	b, err := s.Create("x", root, "n", Manual)
	if err != nil {
		t.Fatal(err)
	}
	cur, _ := Scan(root, true)
	if !b.Matches(cur) {
		t.Fatal("identical save should match")
	}
	writeTree(t, root, map[string]string{"b": "3"})
	cur, _ = Scan(root, true)
	if b.Matches(cur) {
		t.Fatal("changed save should not match")
	}
}
