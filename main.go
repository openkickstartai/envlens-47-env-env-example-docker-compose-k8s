package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

type EnvRef struct {
	Name, Source, File, Default string
	Line                       int
}

type Issue struct {
	Type, Var, Detail string
}

func main() {
	dir := "."
	ciMode := false
	for _, a := range os.Args[1:] {
		switch a {
		case "--ci":
			ciMode = true
		case "scan":
			continue
		default:
			dir = a
		}
	}
	refs, err := ScanDir(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	issues := DetectDrift(refs)
	printReport(refs, issues)
	if ciMode && len(issues) > 0 {
		os.Exit(1)
	}
}

func DetectDrift(refs []EnvRef) []Issue {
	byCategory := map[string]map[string]bool{}
	for _, r := range refs {
		cat := r.Source
		if byCategory[cat] == nil {
			byCategory[cat] = map[string]bool{}
		}
		byCategory[cat][r.Name] = true
	}
	codeVars := byCategory["code"]
	exampleVars := byCategory["envfile"]
	var issues []Issue
	for v := range codeVars {
		if exampleVars != nil && !exampleVars[v] {
			issues = append(issues, Issue{"missing", v, "Referenced in code but missing from .env.example"})
		}
	}
	for v := range exampleVars {
		if codeVars != nil && !codeVars[v] {
			issues = append(issues, Issue{"ghost", v, "In .env.example but never referenced in code"})
		}
	}
	sensitive := []string{"PASSWORD", "SECRET", "TOKEN", "KEY", "PRIVATE"}
	for _, r := range refs {
		if r.Source != "docker-compose" {
			continue
		}
		upper := strings.ToUpper(r.Name)
		for _, s := range sensitive {
			if strings.Contains(upper, s) {
				issues = append(issues, Issue{"sensitive", r.Name, fmt.Sprintf("Sensitive var exposed in %s:%d", r.File, r.Line)})
				break
			}
		}
	}
	// Detect default value inconsistencies across sources
	defaults := map[string]map[string]string{}
	for _, r := range refs {
		if r.Default == "" {
			continue
		}
		if defaults[r.Name] == nil {
			defaults[r.Name] = map[string]string{}
		}
		defaults[r.Name][r.Source] = r.Default
	}
	for v, srcDefaults := range defaults {
		vals := map[string]bool{}
		for _, d := range srcDefaults {
			vals[d] = true
		}
		if len(vals) > 1 {
			sources := make([]string, 0, len(srcDefaults))
			for src, d := range srcDefaults {
				sources = append(sources, fmt.Sprintf("%s=%q", src, d))
			}
			sort.Strings(sources)
			issues = append(issues, Issue{"default-mismatch", v, fmt.Sprintf("Inconsistent defaults: %s", strings.Join(sources, ", "))})
		}
	}
	sort.Slice(issues, func(i, j int) bool { return issues[i].Var < issues[j].Var })
	return issues
}

func printReport(refs []EnvRef, issues []Issue) {
	vars := map[string][]string{}
	for _, r := range refs {
		vars[r.Name] = append(vars[r.Name], fmt.Sprintf("%s(%s:%d)", r.Source, r.File, r.Line))
	}
	names := make([]string, 0, len(vars))
	for n := range vars {
		names = append(names, n)
	}
	sort.Strings(names)
	fmt.Printf("=== EnvLens Report ===\nFound %d unique vars in %d refs\n\n", len(names), len(refs))
	for _, n := range names {
		fmt.Printf("  %-28s %s\n", n, strings.Join(vars[n], ", "))
	}
	if len(issues) > 0 {
		fmt.Printf("\n!! %d issues:\n", len(issues))
		for _, i := range issues {
			fmt.Printf("  [%s] %s - %s\n", i.Type, i.Var, i.Detail)
		}
	} else {
		fmt.Println("\nNo drift detected.")
	}
}
