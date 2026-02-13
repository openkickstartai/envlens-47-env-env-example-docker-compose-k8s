package main

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var codePatterns = map[string]*regexp.Regexp{
	".py":   regexp.MustCompile(`os\.(?:environ\.get\(\s*|environ\[\s*|getenv\(\s*)["']([A-Z_][A-Z0-9_]*)["']`),
	".go":   regexp.MustCompile(`os\.(?:Getenv|LookupEnv)\("([A-Z_][A-Z0-9_]*)"\)`),
	".ts":   regexp.MustCompile(`process\.env\.([A-Z_][A-Z0-9_]*)`),
	".js":   regexp.MustCompile(`process\.env\.([A-Z_][A-Z0-9_]*)`),
	".java": regexp.MustCompile(`System\.getenv\("([A-Z_][A-Z0-9_]*)"\)`),
}

var (
	envLineRe    = regexp.MustCompile(`^([A-Z_][A-Z0-9_]*)=(.*)`)
	composeEnvRe = regexp.MustCompile(`^([A-Z_][A-Z0-9_]*)(?:[=:]\s*(.*))?$`)
	skipDirs     = map[string]bool{"vendor": true, "node_modules": true, ".git": true}
)

func ScanDir(dir string) ([]EnvRef, error) {
	var all []EnvRef
	err := filepath.Walk(dir, func(p string, fi os.FileInfo, e error) error {
		if e != nil {
			return e
		}
		if fi.IsDir() {
			if skipDirs[fi.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		name := fi.Name()
		ext := filepath.Ext(name)
		if re, ok := codePatterns[ext]; ok {
			all = append(all, scanRegex(p, "code", re)...)
		}
		if name == ".env.example" || name == ".env.sample" {
			all = append(all, scanEnv(p, "envfile")...)
		} else if name == ".env" {
			all = append(all, scanEnv(p, "dotenv")...)
		}
		if strings.HasPrefix(name, "docker-compose") && (ext == ".yml" || ext == ".yaml") {
			all = append(all, scanCompose(p)...)
		}
		return nil
	})
	return all, err
}

func scanRegex(path, src string, re *regexp.Regexp) []EnvRef {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var refs []EnvRef
	s := bufio.NewScanner(f)
	for ln := 1; s.Scan(); ln++ {
		for _, m := range re.FindAllStringSubmatch(s.Text(), -1) {
			refs = append(refs, EnvRef{Name: m[1], Source: src, File: path, Line: ln})
		}
	}
	return refs
}

func scanEnv(path, src string) []EnvRef {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var refs []EnvRef
	s := bufio.NewScanner(f)
	for ln := 1; s.Scan(); ln++ {
		t := strings.TrimSpace(s.Text())
		if t == "" || t[0] == '#' {
			continue
		}
		if m := envLineRe.FindStringSubmatch(t); m != nil {
			refs = append(refs, EnvRef{Name: m[1], Source: src, File: path, Line: ln, Default: m[2]})
		}
	}
	return refs
}

func scanCompose(path string) []EnvRef {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var refs []EnvRef
	s := bufio.NewScanner(f)
	inEnv := false
	baseIndent := 0
	for ln := 1; s.Scan(); ln++ {
		text := s.Text()
		trimmed := strings.TrimSpace(text)
		curIndent := len(text) - len(strings.TrimLeft(text, " "))
		if trimmed == "environment:" {
			inEnv = true
			baseIndent = curIndent
			continue
		}
		if !inEnv {
			continue
		}
		if trimmed == "" || trimmed[0] == '#' {
			continue
		}
		if curIndent <= baseIndent {
			inEnv = false
			continue
		}
		clean := strings.TrimSpace(strings.TrimPrefix(trimmed, "-"))
		if m := composeEnvRe.FindStringSubmatch(clean); m != nil {
			refs = append(refs, EnvRef{Name: m[1], Source: "docker-compose", File: path, Line: ln, Default: m[2]})
		}
	}
	return refs
}
