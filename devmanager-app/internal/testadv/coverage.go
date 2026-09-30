// Package testadv implementa las capacidades de testing avanzado del issue
// #70 sin dependencias externas. Nunca lanza procesos: parsea artefactos que
// las herramientas del proyecto ya generaron (coverage, k6, artillery,
// screenshots PNG) y calcula resúmenes/diffs. Stdlib only.
//
// Todo lo que devuelve esta capa son datos agregados: nunca contenido de
// ficheros del usuario (los fixtures se listan por nombre/tamaño, no se leen).
package testadv

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Metric es una métrica de cobertura con sus conteos. Total==0 significa "sin
// elementos" y cuenta como 100% (una métrica vacía no puede incumplir umbral).
type Metric struct {
	Covered int     `json:"covered"`
	Total   int     `json:"total"`
	Pct     float64 `json:"pct"`
}

func newMetric(covered, total int) Metric {
	m := Metric{Covered: covered, Total: total}
	if total <= 0 {
		m.Pct = 100
		return m
	}
	m.Pct = round2(float64(covered) / float64(total) * 100)
	return m
}

func round2(f float64) float64 { return math.Round(f*100) / 100 }

// FileCoverage es la cobertura de un fichero concreto (para señalar los peores).
type FileCoverage struct {
	Path      string `json:"path"`
	Lines     Metric `json:"lines"`
	Branches  Metric `json:"branches"`
	Functions Metric `json:"functions"`
}

// CoverageSummary resume la cobertura del proyecto.
type CoverageSummary struct {
	Source     string  `json:"source"` // go | istanbul | cobertura
	Path       string  `json:"path"`
	Lines      Metric  `json:"lines"`
	Statements Metric  `json:"statements"`
	Functions  Metric  `json:"functions"`
	Branches   Metric  `json:"branches"`
	Files      int     `json:"files"`
	Threshold  float64 `json:"threshold"`
	Pass       bool    `json:"pass"`
}

// WithThreshold fija el umbral y evalúa Pass contra las métricas con elementos.
func (c CoverageSummary) WithThreshold(t float64) CoverageSummary {
	c.Threshold = t
	c.Pass = true
	for _, m := range []Metric{c.Lines, c.Statements, c.Functions, c.Branches} {
		if m.Total == 0 {
			continue
		}
		if m.Pct < t {
			c.Pass = false
		}
	}
	return c
}

// ---- Go coverprofile (go test -coverprofile) ----

// ParseGoCoverProfile parsea un coverprofile de Go:
//
//	mode: set
//	path/file.go:10.2,12.4 3 1
//
// Las líneas se derivan de los statements (el perfil no distingue ambos).
func ParseGoCoverProfile(data []byte) (CoverageSummary, error) {
	lines := strings.Split(string(data), "\n")
	out := CoverageSummary{Source: "go"}
	files := map[string]bool{}
	covered, total := 0, 0
	sawHeader := false
	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "mode:") {
			sawHeader = true
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 3 {
			return CoverageSummary{}, fmt.Errorf("coverprofile línea %d: formato inválido", i+1)
		}
		file := fields[0]
		if idx := strings.LastIndex(file, ":"); idx > 0 {
			file = file[:idx]
		}
		files[file] = true
		n, err := strconv.Atoi(fields[1])
		if err != nil {
			return CoverageSummary{}, fmt.Errorf("coverprofile línea %d: statements inválidos", i+1)
		}
		count, err := strconv.Atoi(fields[2])
		if err != nil {
			return CoverageSummary{}, fmt.Errorf("coverprofile línea %d: count inválido", i+1)
		}
		total += n
		if count > 0 {
			covered += n
		}
	}
	if !sawHeader && total == 0 {
		return CoverageSummary{}, fmt.Errorf("coverprofile sin cabecera mode:")
	}
	out.Files = len(files)
	stmts := newMetric(covered, total)
	out.Statements = stmts
	out.Lines = stmts
	return out, nil
}

// ---- Istanbul coverage-summary.json (nyc / c8) ----

type istanbulCount struct {
	Total   int `json:"total"`
	Covered int `json:"covered"`
}

type istanbulEntry struct {
	Lines      istanbulCount `json:"lines"`
	Statements istanbulCount `json:"statements"`
	Functions  istanbulCount `json:"functions"`
	Branches   istanbulCount `json:"branches"`
}

func isIstanbulMetaKey(key string) bool {
	return key == "total" || key == "branchesTrue"
}

// ParseIstanbulSummary parsea coverage/coverage-summary.json (nyc, c8, jest,
// vitest con reporter json-summary). Ignora la clave "branchesTrue" de nyc.
func ParseIstanbulSummary(data []byte) (CoverageSummary, error) {
	raw := map[string]istanbulEntry{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return CoverageSummary{}, fmt.Errorf("coverage-summary.json: %w", err)
	}
	total, ok := raw["total"]
	if !ok {
		return CoverageSummary{}, fmt.Errorf("coverage-summary.json sin clave \"total\"")
	}
	files := 0
	for k := range raw {
		if !isIstanbulMetaKey(k) {
			files++
		}
	}
	return CoverageSummary{
		Source:     "istanbul",
		Files:      files,
		Lines:      newMetric(total.Lines.Covered, total.Lines.Total),
		Statements: newMetric(total.Statements.Covered, total.Statements.Total),
		Functions:  newMetric(total.Functions.Covered, total.Functions.Total),
		Branches:   newMetric(total.Branches.Covered, total.Branches.Total),
	}, nil
}

// ParseIstanbulFiles devuelve la cobertura por fichero de un
// coverage-summary.json, ordenada de peor a mejor (por líneas) y recortada a
// limit entradas (limit<=0 → todas).
func ParseIstanbulFiles(data []byte) ([]FileCoverage, error) {
	raw := map[string]istanbulEntry{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("coverage-summary.json: %w", err)
	}
	out := make([]FileCoverage, 0, len(raw))
	for k, e := range raw {
		if isIstanbulMetaKey(k) {
			continue
		}
		out = append(out, FileCoverage{
			Path:      k,
			Lines:     newMetric(e.Lines.Covered, e.Lines.Total),
			Branches:  newMetric(e.Branches.Covered, e.Branches.Total),
			Functions: newMetric(e.Functions.Covered, e.Functions.Total),
		})
	}
	return skinniest(out), nil
}

// ---- Istanbul coverage-final.json ----

type istanbulPos struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

type istanbulLoc struct {
	Start istanbulPos `json:"start"`
	End   istanbulPos `json:"end"`
}

type istanbulFinalFile struct {
	Path         string                     `json:"path"`
	StatementMap map[string]istanbulLoc     `json:"statementMap"`
	S            map[string]int             `json:"s"`
	FnMap        map[string]json.RawMessage `json:"fnMap"`
	F            map[string]int             `json:"f"`
	BranchMap    map[string]json.RawMessage `json:"branchMap"`
	B            map[string][]int           `json:"b"`
}

// ParseIstanbulFinal parsea coverage/coverage-final.json (formato completo de
// istanbul/v8): cuenta statements, funciones y branches por fichero.
func ParseIstanbulFinal(data []byte) (CoverageSummary, error) {
	raw := map[string]istanbulFinalFile{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return CoverageSummary{}, fmt.Errorf("coverage-final.json: %w", err)
	}
	if len(raw) == 0 {
		return CoverageSummary{}, fmt.Errorf("coverage-final.json vacío")
	}
	stmtsTotal, stmtsCovered := 0, 0
	fnTotal, fnCovered := 0, 0
	brTotal, brCovered := 0, 0
	lineTotal, lineCovered := map[int]bool{}, map[int]bool{}
	for _, f := range raw {
		stmtsTotal += len(f.S)
		for id, hits := range f.S {
			if hits > 0 {
				stmtsCovered++
				if loc, ok := f.StatementMap[id]; ok {
					lineCovered[loc.Start.Line] = true
				}
			}
		}
		for _, hits := range f.F {
			fnTotal++
			if hits > 0 {
				fnCovered++
			}
		}
		for _, counts := range f.B {
			for _, c := range counts {
				brTotal++
				if c > 0 {
					brCovered++
				}
			}
		}
		for _, loc := range f.StatementMap {
			lineTotal[loc.Start.Line] = true
		}
	}
	return CoverageSummary{
		Source:     "istanbul",
		Files:      len(raw),
		Lines:      newMetric(len(lineCovered), len(lineTotal)),
		Statements: newMetric(stmtsCovered, stmtsTotal),
		Functions:  newMetric(fnCovered, fnTotal),
		Branches:   newMetric(brCovered, brTotal),
	}, nil
}

// ---- Cobertura XML ----

type coberturaXML struct {
	LineRate        string `xml:"line-rate,attr"`
	BranchRate      string `xml:"branch-rate,attr"`
	LinesCovered    string `xml:"lines-covered,attr"`
	LinesValid      string `xml:"lines-valid,attr"`
	BranchesCovered string `xml:"branches-covered,attr"`
	BranchesValid   string `xml:"branches-valid,attr"`
	Packages        struct {
		Package []struct {
			Classes struct {
				Class []struct {
					Methods []struct {
						Lines []struct {
							Hits string `xml:"hits,attr"`
						} `xml:"lines>line"`
					} `xml:"methods>method"`
					Lines []struct {
						Hits string `xml:"hits,attr"`
					} `xml:"lines>line"`
				} `xml:"class"`
			} `xml:"classes"`
		} `xml:"package"`
	} `xml:"packages"`
}

// ParseCobertura parsea un XML de cobertura (cobertura, coverage.py, gocov,
// jest-cobertura). Usa los conteos explícitos cuando existen y las tasas
// (line-rate) como respaldo.
func ParseCobertura(data []byte) (CoverageSummary, error) {
	var doc coberturaXML
	if err := xml.Unmarshal(data, &doc); err != nil {
		return CoverageSummary{}, fmt.Errorf("cobertura XML: %w", err)
	}
	lineCovered, lineTotal := 0, 0
	fnTotal, fnCovered := 0, 0
	classes := 0
	for _, pkg := range doc.Packages.Package {
		for _, cls := range pkg.Classes.Class {
			classes++
			for _, ln := range cls.Lines {
				lineTotal++
				if hits, _ := strconv.Atoi(strings.TrimSpace(ln.Hits)); hits > 0 {
					lineCovered++
				}
			}
			for _, m := range cls.Methods {
				fnTotal++
				hit := false
				for _, ln := range m.Lines {
					if hits, _ := strconv.Atoi(strings.TrimSpace(ln.Hits)); hits > 0 {
						hit = true
					}
				}
				if hit {
					fnCovered++
				}
			}
		}
	}
	out := CoverageSummary{Source: "cobertura", Files: classes}
	// Conteos explícitos (lines-valid/lines-covered) tienen prioridad sobre el
	// recuento de elementos <line> (algunos exporters no los listan todos).
	if c, err1 := strconv.Atoi(doc.LinesCovered); err1 == nil {
		if v, err2 := strconv.Atoi(doc.LinesValid); err2 == nil && v > 0 {
			lineCovered, lineTotal = c, v
		}
	}
	out.Lines = newMetric(lineCovered, lineTotal)
	out.Statements = out.Lines
	out.Functions = newMetric(fnCovered, fnTotal)
	if c, err1 := strconv.Atoi(doc.BranchesCovered); err1 == nil {
		if v, err2 := strconv.Atoi(doc.BranchesValid); err2 == nil && v > 0 {
			out.Branches = newMetric(c, v)
		}
	}
	if out.Branches.Total == 0 {
		if rate, ok := ratePct(doc.BranchRate); ok {
			out.Branches = Metric{Pct: rate}
		}
	}
	return out, nil
}

func ratePct(raw string) (float64, bool) {
	if strings.TrimSpace(raw) == "" {
		return 0, false
	}
	f, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return 0, false
	}
	return round2(f * 100), true
}

// ---- Descubrimiento de artefactos ----

// coverageCandidates son los ficheros de cobertura soportados, en orden de
// preferencia (más específico/richer primero).
var coverageCandidates = []string{
	filepath.Join("coverage", "coverage-summary.json"),
	filepath.Join("coverage", "coverage-final.json"),
	filepath.Join("coverage", "cobertura-coverage.xml"),
	"coverage.xml",
	filepath.Join("coverage", "coverage.out"),
	"coverage.out",
}

// DetectCoverageFile devuelve el primer artefacto de cobertura existente en el
// proyecto (vacío si no hay ninguno).
func DetectCoverageFile(projectPath string) string {
	if strings.TrimSpace(projectPath) == "" {
		return ""
	}
	for _, rel := range coverageCandidates {
		p := filepath.Join(projectPath, rel)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

// LoadCoverage detecta y parsea la cobertura del proyecto. ok=false cuando no
// hay ningún artefacto (no es un error: el usuario aún no corrió coverage).
func LoadCoverage(projectPath string) (CoverageSummary, bool, error) {
	path := DetectCoverageFile(projectPath)
	if path == "" {
		return CoverageSummary{}, false, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return CoverageSummary{}, false, fmt.Errorf("leer cobertura: %w", err)
	}
	name := strings.ToLower(filepath.Base(path))
	var summary CoverageSummary
	switch {
	case name == "coverage-summary.json":
		summary, err = ParseIstanbulSummary(data)
	case name == "coverage-final.json":
		summary, err = ParseIstanbulFinal(data)
	case strings.HasSuffix(name, ".xml"):
		summary, err = ParseCobertura(data)
	default:
		summary, err = ParseGoCoverProfile(data)
	}
	if err != nil {
		return CoverageSummary{}, false, err
	}
	summary.Path = path
	return summary, true, nil
}

// LoadCoverageFiles devuelve los peores ficheros (solo disponible con
// coverage-summary.json; el resto de formatos no llevan desglose barato).
func LoadCoverageFiles(projectPath string, limit int) []FileCoverage {
	path := DetectCoverageFile(projectPath)
	if path == "" || !strings.HasSuffix(strings.ToLower(path), "coverage-summary.json") {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	files, err := ParseIstanbulFiles(data)
	if err != nil {
		return nil
	}
	if limit > 0 && len(files) > limit {
		files = files[:limit]
	}
	return files
}

// skinniest ordena por cobertura de líneas asc y desempata por path.
func skinniest(files []FileCoverage) []FileCoverage {
	sort.Slice(files, func(i, j int) bool {
		if files[i].Lines.Pct != files[j].Lines.Pct {
			return files[i].Lines.Pct < files[j].Lines.Pct
		}
		return files[i].Path < files[j].Path
	})
	return files
}
