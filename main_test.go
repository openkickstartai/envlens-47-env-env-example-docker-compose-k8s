package main

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestScanPythonEnvVars(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "app.py", "import os\ndb = os.getenv(\"DB_HOST\")\nport = os.environ[\"DB_PORT\"]\nsecret = os.environ.get(\"APP_SECRET\")\n")
	refs, err := ScanDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, r := range refs {
		names[r.Name] = true
		if r.Source != "code" {
			t.Errorf("expected source 'code', got %q", r.Source)
		}
	}
	for _, want := range []string{"DB_HOST", "DB_PORT", "APP_SECRET"} {
		if !names[want] {
			t.Errorf("expected %s to be found", want)
		}
	}
}

func TestDriftMissingFromExample(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "app.py", "import os\nx = os.getenv(\"API_KEY\")\ny = os.getenv(\"DB_HOST\")\n")
	writeFile(t, dir, ".env.example", "DB_HOST=localhost\n")
	refs, _ := ScanDir(dir)
	issues := DetectDrift(refs)
	found := false
	for _, i := range issues {
		if i.Type == "missing" && i.Var == "API_KEY" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'missing' issue for API_KEY")
	}
}

func TestDriftGhostVar(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "app.py", "import os\nx = os.getenv(\"DB_HOST\")\n")
	writeFile(t, dir, ".env.example", "DB_HOST=localhost\nOLD_VAR=removed\n")
	refs, _ := ScanDir(dir)
	issues := DetectDrift(refs)
	found := false
	for _, i := range issues {
		if i.Type == "ghost" && i.Var == "OLD_VAR" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'ghost' issue for OLD_VAR")
	}
}

func TestSensitiveVarInCompose(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "docker-compose.yml", "services:\n  app:\n    environment:\n      - DB_PASSWORD=secret123\n      - APP_PORT=8080\n")
	refs, _ := ScanDir(dir)
	issues := DetectDrift(refs)
	sensitiveFound := false
	falsePositive := false
	for _, i := range issues {
		if i.Type == "sensitive" && i.Var == "DB_PASSWORD" {
			sensitiveFound = true
		}
		if i.Type == "sensitive" && i.Var == "APP_PORT" {
			falsePositive = true
		}
	}
	if !sensitiveFound {
		t.Error("expected 'sensitive' issue for DB_PASSWORD")
	}
	if falsePositive {
		t.Error("APP_PORT should not be flagged as sensitive")
	}
}

func TestScanGoAndTS(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "main.go", "package main\nimport \"os\"\nvar h = os.Getenv(\"GO_HOST\")\n")
	writeFile(t, dir, "index.ts", "const p = process.env.TS_PORT\n")
	refs, _ := ScanDir(dir)
	names := map[string]bool{}
	for _, r := range refs {
		names[r.Name] = true
	}
	if !names["GO_HOST"] {
		t.Error("expected GO_HOST")
	}
	if !names["TS_PORT"] {
		t.Error("expected TS_PORT")
	}
}
