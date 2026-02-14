package main

import (
	"testing"
)

// helper: find issue by type+var in slice
func findIssue(issues []Issue, typ, varName string) *Issue {
	for _, i := range issues {
		if i.Type == typ && i.Var == varName {
			return &i
		}
	}
	return nil
}

func countByType(issues []Issue, typ string) int {
	n := 0
	for _, i := range issues {
		if i.Type == typ {
			n++
		}
	}
	return n
}

func TestDetectDrift_EmptyInput(t *testing.T) {
	issues := DetectDrift(nil)
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues for nil input, got %d", len(issues))
	}
	issues = DetectDrift([]EnvRef{})
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues for empty input, got %d", len(issues))
	}
}

func TestDetectDrift_MissingFromEnvExample(t *testing.T) {
	refs := []EnvRef{
		{Name: "DB_HOST", Source: "code", File: "main.py", Line: 10},
		{Name: "DB_PORT", Source: "code", File: "main.py", Line: 11},
		{Name: "DB_HOST", Source: "envfile", File: ".env.example", Line: 1},
	}
	issues := DetectDrift(refs)
	if got := findIssue(issues, "missing", "DB_PORT"); got == nil {
		t.Error("expected missing issue for DB_PORT")
	}
	if got := findIssue(issues, "missing", "DB_HOST"); got != nil {
		t.Error("DB_HOST is in both sources, should not be missing")
	}
}

func TestDetectDrift_GhostVar(t *testing.T) {
	refs := []EnvRef{
		{Name: "DB_HOST", Source: "code", File: "main.py", Line: 10},
		{Name: "DB_HOST", Source: "envfile", File: ".env.example", Line: 1},
		{Name: "LEGACY_FLAG", Source: "envfile", File: ".env.example", Line: 2},
	}
	issues := DetectDrift(refs)
	if got := findIssue(issues, "ghost", "LEGACY_FLAG"); got == nil {
		t.Error("expected ghost issue for LEGACY_FLAG")
	}
	if got := findIssue(issues, "ghost", "DB_HOST"); got != nil {
		t.Error("DB_HOST is referenced in code, should not be ghost")
	}
}

func TestDetectDrift_SensitiveInDockerCompose(t *testing.T) {
	refs := []EnvRef{
		{Name: "DB_PASSWORD", Source: "docker-compose", File: "docker-compose.yml", Line: 5},
		{Name: "APP_PORT", Source: "docker-compose", File: "docker-compose.yml", Line: 6},
	}
	issues := DetectDrift(refs)
	if got := findIssue(issues, "sensitive", "DB_PASSWORD"); got == nil {
		t.Error("expected sensitive issue for DB_PASSWORD")
	}
	if got := findIssue(issues, "sensitive", "APP_PORT"); got != nil {
		t.Error("APP_PORT should not be flagged as sensitive")
	}
}

func TestDetectDrift_AllSensitivePatterns(t *testing.T) {
	vars := []string{"API_KEY", "AUTH_TOKEN", "DB_SECRET", "SSH_PRIVATE_KEY", "USER_PASSWORD"}
	for _, v := range vars {
		refs := []EnvRef{{Name: v, Source: "docker-compose", File: "docker-compose.yml", Line: 1}}
		issues := DetectDrift(refs)
		if findIssue(issues, "sensitive", v) == nil {
			t.Errorf("expected %s to be flagged as sensitive", v)
		}
	}
}

func TestDetectDrift_SensitiveCaseInsensitive(t *testing.T) {
	refs := []EnvRef{
		{Name: "db_password", Source: "docker-compose", File: "docker-compose.yml", Line: 1},
	}
	issues := DetectDrift(refs)
	if findIssue(issues, "sensitive", "db_password") == nil {
		t.Error("lowercase sensitive var should still be detected")
	}
}

func TestDetectDrift_SensitiveNotFlaggedOutsideDockerCompose(t *testing.T) {
	refs := []EnvRef{
		{Name: "DB_PASSWORD", Source: "code", File: "main.py", Line: 5},
		{Name: "DB_PASSWORD", Source: "envfile", File: ".env.example", Line: 1},
	}
	issues := DetectDrift(refs)
	if countByType(issues, "sensitive") > 0 {
		t.Error("sensitive should only be flagged for docker-compose source")
	}
}

func TestDetectDrift_NoIssuesWhenFullyConsistent(t *testing.T) {
	refs := []EnvRef{
		{Name: "DB_HOST", Source: "code", File: "main.py", Line: 10, Default: "localhost"},
		{Name: "DB_HOST", Source: "envfile", File: ".env.example", Line: 1, Default: "localhost"},
		{Name: "DB_PORT", Source: "code", File: "main.py", Line: 11},
		{Name: "DB_PORT", Source: "envfile", File: ".env.example", Line: 2},
	}
	issues := DetectDrift(refs)
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues, got %d: %+v", len(issues), issues)
	}
}

func TestDetectDrift_OnlyCodeVars_NoEnvFile(t *testing.T) {
	refs := []EnvRef{
		{Name: "DB_HOST", Source: "code", File: "main.py", Line: 10},
		{Name: "DB_PORT", Source: "code", File: "main.py", Line: 11},
	}
	issues := DetectDrift(refs)
	if countByType(issues, "missing") > 0 {
		t.Error("no envfile present, should not report missing issues")
	}
}

func TestDetectDrift_OnlyEnvFile_NoCode(t *testing.T) {
	refs := []EnvRef{
		{Name: "DB_HOST", Source: "envfile", File: ".env.example", Line: 1},
	}
	issues := DetectDrift(refs)
	if countByType(issues, "ghost") > 0 {
		t.Error("no code present, should not report ghost issues")
	}
}

func TestDetectDrift_DefaultValueMismatch(t *testing.T) {
	refs := []EnvRef{
		{Name: "DB_PORT", Source: "code", File: "main.py", Line: 10, Default: "5432"},
		{Name: "DB_PORT", Source: "envfile", File: ".env.example", Line: 1, Default: "3306"},
		{Name: "DB_PORT", Source: "docker-compose", File: "docker-compose.yml", Line: 5, Default: "5432"},
	}
	issues := DetectDrift(refs)
	if got := findIssue(issues, "default-mismatch", "DB_PORT"); got == nil {
		t.Fatal("expected default-mismatch issue for DB_PORT")
	}
}

func TestDetectDrift_DefaultValueConsistent(t *testing.T) {
	refs := []EnvRef{
		{Name: "DB_PORT", Source: "code", File: "main.py", Line: 10, Default: "5432"},
		{Name: "DB_PORT", Source: "envfile", File: ".env.example", Line: 1, Default: "5432"},
	}
	issues := DetectDrift(refs)
	if findIssue(issues, "default-mismatch", "DB_PORT") != nil {
		t.Error("should not flag default-mismatch when defaults are identical")
	}
}

func TestDetectDrift_IssuesSortedByVarName(t *testing.T) {
	refs := []EnvRef{
		{Name: "Z_VAR", Source: "code", File: "main.py", Line: 1},
		{Name: "A_VAR", Source: "code", File: "main.py", Line: 2},
		{Name: "M_VAR", Source: "envfile", File: ".env.example", Line: 1},
		{Name: "B_VAR", Source: "envfile", File: ".env.example", Line: 2},
	}
	issues := DetectDrift(refs)
	if len(issues) < 2 {
		t.Skip("need at least 2 issues to verify sort")
	}
	for i := 1; i < len(issues); i++ {
		if issues[i].Var < issues[i-1].Var {
			t.Errorf("issues not sorted: %q came after %q", issues[i].Var, issues[i-1].Var)
		}
	}
}

func TestDetectDrift_DuplicateCodeRefsNoDuplicateIssues(t *testing.T) {
	refs := []EnvRef{
		{Name: "API_URL", Source: "code", File: "main.py", Line: 5},
		{Name: "API_URL", Source: "code", File: "handler.py", Line: 12},
		{Name: "OTHER", Source: "envfile", File: ".env.example", Line: 1},
	}
	issues := DetectDrift(refs)
	count := 0
	for _, i := range issues {
		if i.Type == "missing" && i.Var == "API_URL" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 missing issue for API_URL, got %d", count)
	}
}
