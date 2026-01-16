package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// TableResult represents the parsed result for a table
type TableResult struct {
	Schema           string
	Table            string
	FullName         string
	IsEquivalent     bool
	IsStructureEqual bool
	DataDiffRows     string
	UpCount          int
	DownCount        int
	Result           string
}

type SyncDiffOutput struct {
	EquivalentTables   []TableResult `json:"equivalent_tables"`
	InconsistentTables []TableResult `json:"inconsistent_tables"`
	TotalEquivalent    int           `json:"total_equivalent"`
	TotalInconsistent  int           `json:"total_inconsistent"`
	AllEquivalent      bool          `json:"all_equivalent"`
}

// ParseSummary parses the sync_diff_inspector summary file
func ParseSummary(filePath string) ([]TableResult, []TableResult, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()

	var equivalentTables []TableResult
	var inconsistentTables []TableResult
	scanner := bufio.NewScanner(file)

	// Regex patterns
	tablePattern := regexp.MustCompile(`\` + "`" + `([^` + "`" + `]+)` + "`" + `\.` + "`" + `([^` + "`" + `]+)` + "`")
	// sectionPattern := regexp.MustCompile(`^The (table structure and data|following tables)`)
	// tableRowPattern := regexp.MustCompile(`^\| .*` + "`" + `[^` + "`" + `]+` + "`" + `\.` + "`" + `[^` + "`" + `]+` + "`")

	currentSection := ""
	skipNextSeparator := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Detect section headers
		if strings.Contains(line, "The table structure and data in following tables are equivalent") {
			currentSection = "equivalent"
			skipNextSeparator = true
			continue
		}
		if strings.Contains(line, "The following tables contains inconsistent data") {
			currentSection = "inconsistent"
			skipNextSeparator = true
			continue
		}

		// Skip separator lines after section headers
		if skipNextSeparator && strings.HasPrefix(line, "+--") {
			skipNextSeparator = false
			continue
		}

		// Skip header lines within tables
		if strings.HasPrefix(line, "| TABLE ") || strings.HasPrefix(line, "| TABLE |") {
			continue
		}

		// Parse table rows
		if tablePattern.MatchString(line) {
			table := parseTableRow(line, currentSection)
			if table != nil {
				if currentSection == "equivalent" {
					equivalentTables = append(equivalentTables, *table)
				} else if currentSection == "inconsistent" {
					inconsistentTables = append(inconsistentTables, *table)
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, nil, err
	}

	return equivalentTables, inconsistentTables, nil
}

// parseTableRow parses a single table row from the summary
func parseTableRow(line, section string) *TableResult {
	// Extract table name
	tableRe := regexp.MustCompile(`\` + "`" + `([^` + "`" + `]+)` + "`" + `\.` + "`" + `([^` + "`" + `]+)` + "`")
	matches := tableRe.FindStringSubmatch(line)
	if len(matches) != 3 {
		return nil
	}

	schema, table := matches[1], matches[2]
	result := &TableResult{
		Schema:   schema,
		Table:    table,
		FullName: fmt.Sprintf("%s.%s", schema, table),
	}

	// Split line by pipe and clean up
	parts := strings.Split(line, "|")
	if len(parts) < 2 {
		return result
	}

	// Clean parts (trim spaces)
	cleanParts := make([]string, 0, len(parts))
	for _, part := range parts {
		cleanParts = append(cleanParts, strings.TrimSpace(part))
	}

	switch section {
	case "equivalent":
		result.IsEquivalent = true
		result.IsStructureEqual = true
		// For equivalent tables, upcount and downcount are at the end
		if len(cleanParts) >= 4 {
			fmt.Sscanf(cleanParts[len(cleanParts)-2], "%d", &result.UpCount)
			fmt.Sscanf(cleanParts[len(cleanParts)-1], "%d", &result.DownCount)
		}
	case "inconsistent":
		result.IsEquivalent = false
		// Parse inconsistent table row
		if len(cleanParts) >= 8 {
			result.Result = cleanParts[2]
			result.IsStructureEqual = strings.ToLower(cleanParts[3]) == "true"
			result.DataDiffRows = cleanParts[4]
			fmt.Sscanf(cleanParts[5], "%d", &result.UpCount)
			fmt.Sscanf(cleanParts[6], "%d", &result.DownCount)
		}
	}

	return result
}

// PrintResults displays the parsed results
func PrintResults(equivalent, inconsistent []TableResult) {
	fmt.Println("=== EQUIVALENT TABLES ===")
	fmt.Printf("Count: %d\n\n", len(equivalent))
	for _, t := range equivalent {
		fmt.Printf("  %s.%s\n", t.Schema, t.Table)
	}

	fmt.Println("\n=== INCONSISTENT TABLES ===")
	fmt.Printf("Count: %d\n\n", len(inconsistent))
	for _, t := range inconsistent {
		fmt.Printf("  %s.%s\n", t.Schema, t.Table)
		fmt.Printf("    Structure Equal: %v\n", t.IsStructureEqual)
		fmt.Printf("    Data Diff Rows: %s\n", t.DataDiffRows)
		fmt.Printf("    Up Count: %d, Down Count: %d\n", t.UpCount, t.DownCount)
		fmt.Printf("    Result: %s\n\n", t.Result)
	}
}
func ParseSyncDiffOutput(summaryFile string) (*SyncDiffOutput, error) {
	equivalent, inconsistent, err := ParseSummary(summaryFile)
	if err != nil {
		fmt.Printf("Error parsing summary: %v\n", err)
		return nil, err
	}

	PrintResults(equivalent, inconsistent)

	// Example: Create JSON output
	fmt.Println("\n=== JSON OUTPUT ===")
	output := &SyncDiffOutput{
		EquivalentTables:   equivalent,
		InconsistentTables: inconsistent,
		TotalEquivalent:    len(equivalent),
		TotalInconsistent:  len(inconsistent),
		AllEquivalent:      len(inconsistent) == 0,
	}

	// You can marshal this to JSON
	jsonData, _ := json.MarshalIndent(output, "", "  ")
	fmt.Println(string(jsonData))

	fmt.Printf("\nSummary: %d equivalent, %d inconsistent, All Equivalent: %v\n",
		len(equivalent), len(inconsistent), len(inconsistent) == 0)

	return output, nil
}
