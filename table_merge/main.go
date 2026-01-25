package main

import (
	"bytes"
	"context"
	"crypto/md5"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"text/template"

	openai "github.com/sashabaranov/go-openai"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	_ "github.com/go-sql-driver/mysql"
	selector "github.com/pingcap/tidb/pkg/util/table-rule-selector"
)

type DBConnInfo struct {
	Name     string   `yaml:"Name"`
	Host     string   `yaml:"Host"`
	Port     int      `yaml:"Port"`
	User     string   `yaml:"User"`
	Password string   `yaml:"Password"`
	DBs      []string `yaml:"DBs"`
}

type TableInfo struct {
	MD5Columns          string
	MD5ColumnsWithTypes string
	SrcRegex            string
	SrcTableInfo        []string
	DestTableInfo       []string
	DestHasSource       bool
	DestHasSchema       bool
	DestHasTableName    bool
}

var (
	opsType    string
	strTpl     string
	srcDBInfo  DBConnInfo
	destDBInfo DBConnInfo
	outputFile string
	outputErr  string
	configFile string
	llmProduct string
)

var rootCmd = &cobra.Command{
	Use:   "md-toolkit",
	Short: "Toolkit to help DM",
	Run: func(cmd *cobra.Command, args []string) {
		// Exit if help flag is provided
		if cmd.Flag("help").Value.String() == "true" {
			os.Exit(0)
		}

		if llmProduct != "openai" && llmProduct != "deepseek" {
			fmt.Printf("Error: LLM product must be either 'openai' or 'deepseek'\n")
			os.Exit(1)
		}
	},
}

func init() {
	// cobra.OnInitialize(initConfig)

	// Add the --config flag to the root command.
	rootCmd.PersistentFlags().StringVarP(&strTpl, "template", "t", "", "template command for dumpling")

	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "", "Config file")
	rootCmd.PersistentFlags().StringVarP(&llmProduct, "llm", "a", "", "LLM product(openai,deepseek)")

	// Define flags for source and destination databases
	rootCmd.PersistentFlags().StringVar(&opsType, "ops-type", "", "OPS type[sourceAnalyze, generateDumpling, generateSyncDiffconfig, generateMapping, generateDMConfig]")

	rootCmd.PersistentFlags().StringVar(&srcDBInfo.Host, "src-host", "", "Source database host")
	rootCmd.PersistentFlags().IntVar(&srcDBInfo.Port, "src-port", 4000, "Source database port")
	rootCmd.PersistentFlags().StringVar(&srcDBInfo.User, "src-user", "", "Source database user")
	rootCmd.PersistentFlags().StringVar(&srcDBInfo.Password, "src-password", "", "Source database password")
	// rootCmd.PersistentFlags().StringVar(&srcDBInfo.DBName, "src-dbs", "", "Source database name")

	rootCmd.PersistentFlags().StringVar(&destDBInfo.Host, "dest-host", "", "Destination database host")
	rootCmd.PersistentFlags().IntVar(&destDBInfo.Port, "dest-port", 4000, "Destination database port")
	rootCmd.PersistentFlags().StringVar(&destDBInfo.User, "dest-user", "", "Destination database user")
	rootCmd.PersistentFlags().StringVar(&destDBInfo.Password, "dest-password", "", "Destination database password")
	// rootCmd.PersistentFlags().StringVar(&destDBInfo.DBName, "dest-dbs", "", "Destination database name")

	rootCmd.PersistentFlags().StringVar(&outputFile, "output", "", "Output file path")
	rootCmd.PersistentFlags().StringVar(&outputFile, "error-file", "", "Output file path for failed mapping tables")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}

	var config Config
	var err error
	if configFile != "" {
		// fmt.Printf("Config file: %s \n", configFile)
		config, err = readConfig(configFile)
		if err != nil {
			log.Fatalf("Failed to read config file: %v", err)
		}
		// fmt.Printf("the config : %#v \n", config)
	}

	if opsType == "" {
		fmt.Printf("Please provide ops type. \n")
		return
	}

	tableStructure := []TableInfo{}

	/*
			Fetch source database table definitions and create a mapping where:
		    - Key: MD5 hash of consolidated column definitions
		    - Value: List of table names sharing the same column structure
	*/
	for _, sourceDB := range config.SourceDB {
		err := fetch_table_def("source", &tableStructure, sourceDB)
		if err != nil {
			fmt.Printf("Failed to fetch table definition: %v \n", err)
			return
		}
	}
	// err := fetch_table_def("source", &tableStructure, srcDBInfo, strings.Split(srcDBInfo.DBName, ","))
	// if err != nil {
	// 	fmt.Printf("Failed to fetch table definition: %v \n", err)
	// 	return
	// }
	/*
			Similarly, fetch destination database table definitions and create a mapping where:
		    - Key: MD5 hash of consolidated column definitions
		    - Value: List of table names sharing the same column structure
	*/
	err = fetch_table_def("dest", &tableStructure, config.DestDB)
	if err != nil {
		fmt.Printf("Failed to fetch table definition: %v \n", err)
		return
	}
	// }

	// fmt.Printf("template: %s \n", config.Template)
	tmpl := template.Must(template.New("dumpling").Parse(config.Template))

	// Open the output file for writing if specified.
	// Create file handlers for all the source db which will be used to output the dumpling command.
	mapWriter := make(map[string]*os.File)
	for _, db := range config.SourceDB {
		// Open output file for writing if specified
		var outputWriter *os.File
		if outputFile != "" {
			var err error
			outputWriter, err = os.Create(fmt.Sprintf("%s/%s.txt", outputFile, db.Name))
			if err != nil {
				log.Fatalf("Failed to create output file: %v", err)
			}
			defer outputWriter.Close()
		} else {
			outputWriter = os.Stdout
		}
		mapWriter[db.Name] = outputWriter
	}

	// File handler for error output
	var errorWriter *os.File
	if outputErr != "" {
		var err error
		errorWriter, err = os.Create(outputErr)
		if err != nil {
			log.Fatalf("Failed to create output file: %v", err)
		}
		defer errorWriter.Close()
	} else {
		errorWriter = os.Stdout
	}

	// Convert the tableInfo like source: ["TableA, TableB01, TableB02"]  dest: ["TableA, TableB"]
	// to Source: [TableA], Dest: [TableA]
	//   and Source: [TableB01, TableB02], Dest: [TableB]
	// If both source and dest has multiple tables, separate those table with same name.
	// Multiple to multiple can not be handle. Use the name format to make the mapping between the source and destination.
	convertedTableStructure := []TableInfo{}
	for _, tableInfo := range tableStructure {
		// Skip if one to one
		if len(tableInfo.SrcTableInfo) <= 1 || len(tableInfo.DestTableInfo) <= 1 {
			convertedTableStructure = append(convertedTableStructure, tableInfo)
			continue
		}

		// If multiple to multiple, separate them as one-to-one mapping and many-to-one mapping
		if len(tableInfo.SrcTableInfo) > 1 && len(tableInfo.DestTableInfo) > 1 {
			foundTable := []string{}
			// If the table name is same, then we will separate them as one-to-one mapping
			for _, srcTable := range tableInfo.SrcTableInfo {
				for _, destTable := range tableInfo.DestTableInfo {
					if (strings.Split(srcTable, "."))[2] == (strings.Split(destTable, "."))[2] {
						// fmt.Printf("Same table name with layout: %s vs %s \n", srcTable, destTable)
						convertedTableStructure = append(convertedTableStructure, TableInfo{
							MD5Columns:          tableInfo.MD5Columns,
							MD5ColumnsWithTypes: tableInfo.MD5ColumnsWithTypes,
							SrcTableInfo:        []string{srcTable},
							DestTableInfo:       []string{destTable},
						})
						foundTable = append(foundTable, srcTable)
					}
				}
			}

			// If the table name is not same, then we will separate them as many-to-one mapping
			tmpSrcTable := []string{}
			tmpDestTable := []string{}
			for _, srcTable := range tableInfo.SrcTableInfo {
				isFound := false
				for _, foundSrc := range foundTable {
					if srcTable == foundSrc {
						isFound = true
						break
					}
				}
				if !isFound {
					tmpSrcTable = append(tmpSrcTable, srcTable)
				}
			}

			// Find the dest table that has the same base name as the srcTable that was found
			for _, destTable := range tableInfo.DestTableInfo {
				isFound := false
				for _, foundSrc := range foundTable {
					// Check against the base name of the srcTable that was found
					if (strings.Split(destTable, "."))[2] == (strings.Split(foundSrc, "."))[2] {
						isFound = true
						break
					}
				}
				if !isFound {
					tmpDestTable = append(tmpDestTable, destTable)
				}
			}

			// If there are remaining src and dest tables, add them to the convertedTableStructure
			if len(tmpSrcTable) > 0 && len(tmpDestTable) > 0 {
				convertedTableStructure = append(convertedTableStructure, TableInfo{
					MD5Columns:          tableInfo.MD5Columns,
					MD5ColumnsWithTypes: tableInfo.MD5ColumnsWithTypes,
					SrcTableInfo:        tmpSrcTable,
					DestTableInfo:       tmpDestTable,
				})
			}
		}
	}

	tableStructure = convertedTableStructure

	if opsType == "sourceAnalyze" {
		// fmt.Printf("Starting to analyze the source table and check the table structure \n")
		// fmt.Printf("%#v \n", tableStructure)
		fmt.Printf("---------- Pattern 01: one-to-one pattern \n")
		for idx, table := range tableStructure {
			// fmt.Printf("%#v \n", table)
			if len(table.SrcTableInfo) == 1 && len(table.DestTableInfo) == 1 {
				// fmt.Printf("idx: %d, md5: %s, md5 with type: %s, source table: %#v, dest tables: %#v \n", idx, table.MD5Columns, table.MD5ColumnsWithTypes, table.SrcTableInfo, table.DestTableInfo)
				fmt.Printf("idx: %d, md5: %s, md5 with type: %s, Source table: %#v, Dest Table: %#v \n", idx, table.MD5Columns, table.MD5ColumnsWithTypes, table.SrcTableInfo[0], table.DestTableInfo[0])
			}

		}

		fmt.Printf("\n\n---------- Pattern 02: multiple-to-one pattern(No PK conflict) \n")
		for idx, table := range tableStructure {
			if len(table.SrcTableInfo) > 1 && len(table.DestTableInfo) == 1 && (!table.DestHasTableName && !table.DestHasSchema && !table.DestHasSource) {
				// fmt.Printf("idx: %d, md5: %s, md5 with type: %s, source table: %#v, dest tables: %#v \n", idx, table.MD5Columns, table.MD5ColumnsWithTypes, table.SrcTableInfo, table.DestTableInfo)
				fmt.Printf("idx: %d, md5: %s, md5 with type: %s, Source table(%d): %s ..., Dest Table: %s \n", idx, table.MD5Columns, table.MD5ColumnsWithTypes, len(table.SrcTableInfo), table.SrcTableInfo[0], table.SrcTableInfo[0])
			}
		}

		fmt.Printf("\n\n---------- Pattern 03: multiple-to-one pattern(PK conflict) \n")
		for idx, table := range tableStructure {
			if len(table.SrcTableInfo) > 1 && len(table.DestTableInfo) == 1 && (table.DestHasTableName || table.DestHasSchema || table.DestHasSource) {
				// fmt.Printf("idx: %d, md5: %s, md5 with type: %s, source table: %#v, dest tables: %#v \n", idx, table.MD5Columns, table.MD5ColumnsWithTypes, table.SrcTableInfo, table.DestTableInfo)
				fmt.Printf("idx: %d, md5: %s, md5 with type: %s, Source table(%d): %s ..., Dest Table: %s \n", idx, table.MD5Columns, table.MD5ColumnsWithTypes, len(table.SrcTableInfo), table.SrcTableInfo[0], table.DestTableInfo[0])
			}
		}

		fmt.Printf("\n\n---------- Pattern 04: multiple-to-multiple pattern \n")
		for idx, table := range tableStructure {
			if len(table.SrcTableInfo) > 1 && len(table.DestTableInfo) > 1 {
				// fmt.Printf("idx: %d, md5: %s, md5 with type: %s, source table: %#v, dest tables: %#v \n", idx, table.MD5Columns, table.MD5ColumnsWithTypes, table.SrcTableInfo, table.DestTableInfo)
				fmt.Printf("idx: %d, md5: %s, md5 with type: %s, source table: %#v, # of source tables: %#v \n", idx, table.MD5Columns, table.MD5ColumnsWithTypes, table.SrcTableInfo, table.DestTableInfo)
			}

		}
		return
	}

	for _, ti := range tableStructure {
		fmt.Printf("MD5Columns: %s\n", ti.MD5Columns)
		fmt.Printf("MD5ColumnsWithTypes: %s\n", ti.MD5ColumnsWithTypes)
		fmt.Printf("SrcRegex: %s\n", ti.SrcRegex)
		fmt.Printf("SrcTableInfo: %v\n", ti.SrcTableInfo)
		fmt.Printf("DestTableInfo: %v\n", ti.DestTableInfo)
		fmt.Printf("DestHasSource: %t\n", ti.DestHasSource)
		fmt.Printf("DestHasSchema: %t\n", ti.DestHasSchema)
		fmt.Printf("DestHasTableName: %t\n", ti.DestHasTableName)
		fmt.Println("---")
	}

	if opsType == "generateDumpling" {

		// fmt.Printf("--------- Start to prepare dumpling command ----- ---- \n")
		for _, tableInfo := range tableStructure {
			// Case 1: One-to-one mapping
			if len(tableInfo.SrcTableInfo) == 1 && len(tableInfo.DestTableInfo) == 1 {
				srcTable := tableInfo.SrcTableInfo[0]
				// destTable := tableInfo.DestTableInfo[0]
				srcParts := strings.Split(tableInfo.SrcTableInfo[0], ".")
				destParts := strings.Split(tableInfo.DestTableInfo[0], ".")

				sourceData := fetchDumpingSourceData(srcParts[0], srcParts[1], srcParts[2], tableInfo.DestHasSource, tableInfo.DestHasSchema, tableInfo.DestHasTableName)

				dbName := strings.Split(srcTable, ".")[0]
				data := struct {
					SrcTable       string
					DestTable      string
					SrcSchemaName  string
					SrcTableName   string
					DestSchemaName string
					DestTableName  string
					InstanceName   string
					SourceData     string
				}{
					SrcTable:       fmt.Sprintf("%s.%s", srcParts[1], srcParts[2]),
					DestTable:      fmt.Sprintf("%s.%s.{{.Index}}", destParts[1], destParts[2]),
					SrcSchemaName:  srcParts[1],
					SrcTableName:   srcParts[2],
					DestSchemaName: destParts[1],
					DestTableName:  destParts[2],
					InstanceName:   dbName,
					SourceData:     sourceData,
				}

				var buf bytes.Buffer
				if err := tmpl.Execute(&buf, data); err != nil {
					log.Printf("Error executing template: %v", err)
				}
				fmt.Fprintf(mapWriter[dbName], "%s\n", buf.String())
			}

			// Case 2: Many-to-many mapping with same table names and count
			if len(tableInfo.SrcTableInfo) > 1 && len(tableInfo.DestTableInfo) > 1 &&
				len(tableInfo.SrcTableInfo) == len(tableInfo.DestTableInfo) {
				// fmt.Printf("Using table map for multiple tables with same structure\n")
				fmt.Fprintf(errorWriter, "---------- ---------- ---------- ---------- --------------- ---------- ---------- ---------- ----------\n")
				fmt.Fprintf(errorWriter, "| source:      | %s \n", strings.Join(tableInfo.SrcTableInfo, " , "))
				fmt.Fprintf(errorWriter, "| destination: | %s \n", strings.Join(tableInfo.DestTableInfo, " , "))
				fmt.Fprintf(errorWriter, "---------- ---------- ---------- ---------- --------------- ---------- ---------- ---------- ----------\n\n")

				// Match tables by comparing table names after the schema
				for i := 0; i < len(tableInfo.SrcTableInfo); i++ {
					srcParts := strings.Split(tableInfo.SrcTableInfo[i], ".")
					srcTableName := srcParts[len(srcParts)-1]
					dbName := srcParts[0]

					// Find matching destination table
					for j := 0; j < len(tableInfo.DestTableInfo); j++ {
						destParts := strings.Split(tableInfo.DestTableInfo[j], ".")
						destTableName := destParts[len(destParts)-1]

						if srcTableName == destTableName {
							data := struct {
								SrcTable       string
								DestTable      string
								SrcSchemaName  string
								SrcTableName   string
								DestSchemaName string
								DestTableName  string
								InstanceName   string
							}{
								SrcTable:       fmt.Sprintf("%s.%s", srcParts[1], srcParts[2]),
								DestTable:      fmt.Sprintf("%s.%s.{{.Index}}", destParts[1], destParts[2]),
								SrcSchemaName:  srcParts[1],
								SrcTableName:   srcParts[2],
								DestSchemaName: destParts[1],
								DestTableName:  destParts[2],
								InstanceName:   dbName,
							}

							var buf bytes.Buffer
							if err := tmpl.Execute(&buf, data); err != nil {
								log.Printf("Error executing template: %v", err)
							}
							fmt.Fprintf(mapWriter[dbName], "%s\n", buf.String())
							break
						}
					}
				}
			}

			// Case 3: Many-to-one consolidation
			if len(tableInfo.SrcTableInfo) > 1 && len(tableInfo.DestTableInfo) == 1 {
				fmt.Printf("----------------------- \n")
				destTable := tableInfo.DestTableInfo[0]
				destParts := strings.Split(destTable, ".")
				for idx, srcTable := range tableInfo.SrcTableInfo {
					srcParts := strings.Split(srcTable, ".")
					sourceData := fetchDumpingSourceData(srcParts[0], srcParts[1], srcParts[2], tableInfo.DestHasSource, tableInfo.DestHasSchema, tableInfo.DestHasTableName)

					dbName := srcParts[0]
					data := struct {
						SrcTable       string
						DestTable      string
						SrcSchemaName  string
						SrcTableName   string
						DestSchemaName string
						DestTableName  string
						InstanceName   string
						SourceData     string
					}{
						SrcTable:       fmt.Sprintf("%s.%s", srcParts[1], srcParts[2]),
						DestTable:      fmt.Sprintf("%s.%s.%05d{{.Index}}", destParts[1], destParts[2], idx+1),
						SrcSchemaName:  srcParts[1],
						SrcTableName:   srcParts[2],
						DestSchemaName: destParts[1],
						DestTableName:  destParts[2],
						InstanceName:   dbName,
						SourceData:     sourceData,
					}

					var buf bytes.Buffer
					if err := tmpl.Execute(&buf, data); err != nil {
						log.Printf("Error executing template: %v", err)
					}
					fmt.Fprintf(mapWriter[dbName], "%s\n", buf.String())
				}
				// TODO: Implement consolidation logic
			}
		}
	}

	mapPatterns := make(map[string]string)
	if opsType == "generateSyncDiffconfig" {
		var syncDiffOutput *SyncDiffOutput
		if _, err := os.Stat("./output/summary.txt"); err == nil {
			syncDiffOutput, err = ParseSyncDiffOutput("./output/summary.txt")
			if err != nil {
				fmt.Printf("Error parsing sync diff output: %v\n", err)
				return
			}
		}
		for idx := range tableStructure {
			if len(tableStructure[idx].SrcTableInfo) > 2 {
				// Get all source tables except the current one
				allSourceTables := make([]string, 0)
				for i := range tableStructure {
					if i != idx {
						allSourceTables = append(allSourceTables, tableStructure[i].SrcTableInfo...)
					}
				}

				regex, err := generateRegex(tableStructure[idx].SrcTableInfo, allSourceTables, mapPatterns)
				if err != nil {
					fmt.Printf("------ Error generating regex: %v\n", err)
				}

				if regex != nil {
					tableStructure[idx].SrcRegex = *regex
				} else {
					fmt.Printf("Failed to detect the regex\n")
				}
			}
		}

		for _, tableInfo := range tableStructure {

			if tableInfo.SrcRegex != "" {
				fmt.Printf("Using regex for multiple tables: %s -> %s  \n", tableInfo.SrcRegex, tableInfo.DestTableInfo)
			} else {
				fmt.Printf("Mapping Rule: %s -> %s \n", tableInfo.SrcTableInfo, tableInfo.DestTableInfo)
			}
		}

		/****** DM Test ****/
		err := RenderDMSourceConfig(&config)
		if err != nil {
			fmt.Printf("Error rendering DM source config: %v\n", err)
			return
		}
		/****** DM Test ****/

		// Filter tableStructure to keep only those that failed in syncDiffOutput
		if syncDiffOutput != nil && len(syncDiffOutput.InconsistentTables) > 0 {
			filtered := make([]TableInfo, 0, len(tableStructure))
			for _, ti := range tableStructure {
				for _, failed := range syncDiffOutput.InconsistentTables {
					// Match by source table names
					for _, src := range ti.DestTableInfo {
						// Convert from instance.schemaName.tableName to schemaName.tableName
						parts := strings.Split(src, ".")
						if len(parts) == 3 {
							converted := fmt.Sprintf("%s.%s", parts[1], parts[2])
							if converted == failed.FullName {
								filtered = append(filtered, ti)
								break
							}
						}
					}
				}
			}
			err = RenderSyncDiffConfig(&config, &filtered)
			if err != nil {
				fmt.Printf("Error rendering sync diff config: %v\n", err)
				return
			}
		} else {
			err = RenderSyncDiffConfig(&config, &tableStructure)
			if err != nil {
				fmt.Printf("Error rendering sync diff config: %v\n", err)
				return
			}
		}
	}

	if opsType == "generateMapping" {
		for idx := range tableStructure {
			if len(tableStructure[idx].SrcTableInfo) > 2 {
				allSourceTables := make([]string, 0)
				for i := range tableStructure {
					if i != idx {
						allSourceTables = append(allSourceTables, tableStructure[i].SrcTableInfo...)
					}
				}

				regex, err := generateRegex(tableStructure[idx].SrcTableInfo, allSourceTables, mapPatterns)
				if err != nil {
					fmt.Printf("------ Error generating regex: %v\n", err)
				}
				if regex != nil {
					tableStructure[idx].SrcRegex = *regex
				} else {
					fmt.Printf("Failed to detect the regex")
				}
			}
		}

		for _, tableInfo := range tableStructure {
			if tableInfo.SrcRegex != "" {
				fmt.Printf("Using regex for multiple tables: %s \n", tableInfo.SrcRegex)
			}
		}
	}

	return
}

type RuleResult struct {
	Rule string `json:"rule"`
}

func generateGeneralRegex(dataList []string, dataListShouldNotMatch []string) (*string, error) {
	var client *openai.Client
	if llmProduct == "deepseek" {
		config := openai.DefaultConfig(os.Getenv("DEEPSEEK_API_KEY"))
		config.BaseURL = "https://api.deepseek.com/v1"
		client = openai.NewClientWithConfig(config)
	} else {
		client = openai.NewClient(os.Getenv("OPENAI_API_KEY"))
	}

	fmt.Printf("---------- String to extract the regex: %s \n", strings.Join(dataList, ", "))

	tools := []openai.Tool{
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "rule_is_valid",
				Description: "Verify if the given rule matches all required names and excludes others. The rule should match the exact database naming pattern.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"rule": map[string]interface{}{
							"type":        "string",
							"description": "The candidate rule to validate.",
						},
						"dbs_to_match": map[string]interface{}{
							"type": "array",
							"items": map[string]interface{}{
								"type": "string",
							},
							"description": "List of names that the rule MUST match (e.g., 'db_01', 'db_02').",
						},
						"dbs_to_exclude": map[string]interface{}{
							"type": "array",
							"items": map[string]interface{}{
								"type": "string",
							},
							"description": "List of names that the rule MUST NOT match.",
						},
					},
					"required": []string{"rule", "dbs_to_match", "dbs_to_exclude"},
				},
			},
		},
	}

	// System message for database name regex generation
	system := openai.ChatCompletionMessage{
		Role: openai.ChatMessageRoleSystem,
		Content: strings.Join([]string{
			"You are an assistant that generates concise and accurate pattern-matching rules according to the given specification. ",
		}, "\n"),
	}

	samplingNum := calculateSampleSize(len(dataList))
	sampledData := sampleData(dataList, samplingNum)

	fmt.Printf("Sampled data: %s \n", strings.Join(sampledData, ", "))

	user := openai.ChatCompletionMessage{
		Role: openai.ChatMessageRoleUser,
		Content: strings.Join([]string{
			"Rules must follow the pattern specification below:",
			"1. Pattern Characters:",
			"  - '*': Matches zero or more characters (must be the last character)",
			"  - '?': Matches exactly one character",
			"  - '[...]': Matches a single character from the specified range",
			"2. Range Pattern Format:",
			"  - [a-z]: Matches any single character from 'a' to 'z'",
			"  - [!a-z]: Matches any single character NOT in range 'a' to 'z'",
			"  - [abc]: Matches 'a', 'b', or 'c'",
			"3. Limitations:",
			"  - '*' can only appear at the end of the pattern",
			"  - Each '?' matches exactly one character",
			"  - Range patterns are case-sensitive",
			"  - Empty patterns are not allowed",
			"  - Maximum pattern length is not restricted",
			"4. Pattern Types and Examples:",
			"  a. Exact Match:",
			"    - \"abc\" matches exactly \"abc\"",
			"    - \"abd\" matches exactly \"abd\"",
			"  b. Single Character Wildcard (?):",
			"    - \"?bc\" matches \"abc\", \"dbc\"",
			"    - \"a?c\" matches \"abc\", \"adc\"",
			"    - \"ab?\" matches \"abc\", \"abd\"",
			"  c. Multi-Character Wildcard (*):",
			"    - \"ab*\" matches \"abc\", \"abcd\", \"abcde\"",
			"    - \"schema*\" matches \"schema1\", \"schema12\"",
			"    - \"test*\" matches \"test1\", \"test_abc\"",
			"   Note: '*' must be the last character",
			"  d. Character Range ([...]):",
			"    - \"ik[hjkl]\" matches \"ikh\", \"ikj\", \"ikk\", \"ikl\" ",
			"    - \"ik[f-h]\" matches \"ikf\", \"ikg\", \"ikh\"",
			"    - \"i[x-z][1-3]\" matches \"ix1\", \"iy2\", \"iz3\"",
			"  e. Negated Range ([!...]):",
			"    - \"ik[!zxc]\" matches any \"ik\" followed by any character except 'z', 'x', 'c'",
			"    - \"ik[!a-ce-g]\" matches any \"ik\" followed by any character not in ranges a-c and e-g",
			fmt.Sprintf("Create a pattern rule for these sampling values: %s", strings.Join(sampledData, ", ")),
		}, "\n"),
	}

	var model string
	if llmProduct == "deepseek" {
		model = "deepseek-chat"
	} else {
		model = openai.GPT3Dot5Turbo
	}
	messages := []openai.ChatCompletionMessage{system, user}
	const maxRounds = 5
	for round := 1; round <= maxRounds; round++ {
		if round == 4 {
			for _, message := range messages {
				fmt.Printf("%s: %s\n", message.Role, message.Content)
			}
		}

		resp, err := client.CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{
			Model:       model,
			Messages:    messages,
			Temperature: 0.7,
			Tools:       tools,
		})
		if err != nil {
			log.Fatalf("ChatCompletion error (round %d): %v", round, err)
		}

		assistant := resp.Choices[0].Message
		if len(assistant.ToolCalls) > 0 {
			// Add the assistant message with tool_calls to history
			messages = append(messages, assistant)

			for _, tc := range assistant.ToolCalls {
				if tc.Function.Name != "rule_is_valid" {
					continue
				}

				// Parse arguments
				var args RuleResult
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
					// If parsing fails, give the model a helpful error signal
					toolContent := ToolReturn{Valid: false, Error: "Bad JSON arguments for rule_is_valid"}
					contentBytes, _ := json.Marshal(toolContent)
					messages = append(messages, openai.ChatCompletionMessage{
						Role:       openai.ChatMessageRoleTool,
						ToolCallID: tc.ID,
						Content:    string(contentBytes),
					})
					continue
				}

				// Run your local validator
				fmt.Printf("\n\nRule : %s \n", args.Rule)
				toolContent := rule_is_valid(args.Rule, dataList, dataListShouldNotMatch)
				// fmt.Printf("Checking the rule_is_valid tool done %#v \n", toolContent)
				if toolContent.Valid {
					// fmt.Printf("Final regex: %s \n", args.Regex)
					return &toolContent.Rule, nil
				}
				contentBytes, _ := json.Marshal(toolContent)

				messages = append(messages, openai.ChatCompletionMessage{
					Role:       openai.ChatMessageRoleTool,
					ToolCallID: tc.ID,
					Content:    string(contentBytes),
				})
			}

		} else {
			rule := strings.TrimSpace(assistant.Content)
			result := rule_is_valid(rule, dataList, dataListShouldNotMatch)
			if result.Valid {
				return &rule, nil
			}
		}
	}

	fmt.Println("********** Failed: Stopped after max rounds without a final answer. ")

	return nil, fmt.Errorf("failed to generate regex")
}

/*
 * This regex generation is used to detect the tables that are in the same structure for sync_diff_inspector which
 * only allow one routes.rule to compare the data between source tables and destination table. The only one regex is required
 * to conver all the source tables while it should not match any other tables.
 */
func generateRegex(tables []string, tablesShouldNotMatch []string, mapPatterns map[string]string) (*string, error) {
	var err error

	dbList, tableList := splitTables(tables)
	_, tableListExclude := splitTables(tablesShouldNotMatch)

	var dbRegex *string
	if len(dbList) == 1 {
		dbRegex = &dbList[0]
	} else {
		// Calculate MD5 of dbList and lookup in mapPatterns
		dbListStr := strings.Join(dbList, ",")
		dbListMD5 := fmt.Sprintf("%x", md5.Sum([]byte(dbListStr)))
		if cachedRegex, ok := mapPatterns[dbListMD5]; ok {
			dbRegex = &cachedRegex
		} else {
			dbRegex, err = generateGeneralRegex(dbList, nil)
			if err != nil {
				fmt.Printf("Error generating regex for db: %v \n", err)
				return nil, err
			}
			if dbRegex != nil {
				mapPatterns[dbListMD5] = *dbRegex
			}
		}
	}

	var tableRegex *string
	if len(tableList) == 1 {
		tableRegex = &tableList[0]
	} else {
		tableListMD5 := fmt.Sprintf("%x", md5.Sum([]byte(strings.Join(tableList, ","))))
		if cachedRegex, ok := mapPatterns[tableListMD5]; ok {
			tableRegex = &cachedRegex
		} else {
			tableRegex, err = generateGeneralRegex(tableList, tableListExclude)
			if err != nil {
				fmt.Printf("Error generating regex for table: %v \n", err)
				return nil, err
			}
			if tableRegex != nil {
				mapPatterns[tableListMD5] = *tableRegex
			}
		}
	}

	if dbRegex != nil {
		fmt.Printf("Generate db regex: %s \n", *dbRegex)
	} else {
		fmt.Printf("db regex is nil \n")
	}

	if tableRegex != nil {
		fmt.Printf("table regex: %s \n", *tableRegex)
	} else {
		fmt.Printf("table regex is nil \n")
	}

	if dbRegex == nil || tableRegex == nil {
		emptyStr := ""
		return &emptyStr, nil
	}
	regex := fmt.Sprintf("%s.%s", *dbRegex, *tableRegex)

	return &regex, nil
}

type ToolReturn struct {
	Rule           string   `json:"rule"`
	Valid          bool     `json:"valid"`
	MissedMatches  []string `json:"missed_matches"`
	FalsePositives []string `json:"false_positives"`
	Error          string   `json:"error"`
}

func rule_is_valid(pattern string, tables []string, tablesShouldNotMatch []string) ToolReturn {
	result := ToolReturn{
		Rule:           pattern,
		Valid:          true,
		MissedMatches:  []string{},
		FalsePositives: []string{},
		Error:          "",
	}

	// Create a new trie selector
	ts := selector.NewTrieSelector()

	// Create a rule for tables
	// Let's say we want to match all tables that:
	// - Are in schema "mydb"
	schema := "mydb"

	// Define a rule (can be any type)
	rulePattern := struct {
		Action   string
		Priority int
	}{
		Action:   "verification",
		Priority: 1,
	}

	// Insert the rule
	err := ts.Insert(schema, pattern, rulePattern, selector.Insert)
	if err != nil {
		// fmt.Printf("Failed to insert rule: %v\n", err)
		result.Valid = false
		result.Error = fmt.Sprintf("Failed to insert rule: %v\n", err)
		return result
	}

	// Test all the tables which should match the rule
	for _, table := range tables {
		rules := ts.Match(schema, table)
		if rules == nil {
			result.MissedMatches = append(result.MissedMatches, table)
			fmt.Printf("NG: Table %s did not match any rules\n", table)
			result.Valid = false
		}
	}

	// Test all the tables which should not match the rule
	for _, table := range tablesShouldNotMatch {
		rules := ts.Match(schema, table)
		if rules != nil {
			result.FalsePositives = append(result.FalsePositives, table)
			result.Valid = false
			fmt.Printf("NG: Table %s matched! Rules found: %+v\n", table, rules)
		}
	}

	if len(result.MissedMatches) > 0 {
		result.Valid = false
		result.Error = "The validator reports these missed matches (see tool message). You must refine the rule so that all previously provided names match."
	}

	if len(result.FalsePositives) > 0 {
		result.Valid = false
		result.Error = fmt.Sprintf("%s \nThe validator reports false positives (see tool message). You must refine the rule so that all of names are excluded.", result.Error)
	}

	return result
}

func fetch_table_def(tableType string, tableStructure *[]TableInfo, dbInfo DBConnInfo) error {
	// The Data Source Name (DSN) string
	// Format: "user:password@tcp(host:port)/database?param=value"
	// Replace with your actual database credentials
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", dbInfo.User, dbInfo.Password, dbInfo.Host, dbInfo.Port, dbInfo.DBs[0])

	// 1. Open a database handle
	// This does not yet establish a connection, but it prepares the database object.
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("Failed to open database connection: %v", err)
	}
	// Ensure the connection is closed when the main function exits.
	defer db.Close()

	// 2. Ping the database to verify the connection
	// This performs a real check to see if the database is reachable.
	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	// fmt.Println("Successfully connected to the MySQL database!")

	// 2. Define the SQL query with placeholders
	// case when upper(COLUMN_TYPE) IN ('BIGINT', 'INT', 'MEDIUMINT', 'SMALLINT', 'TINYINT') then '0' else NUMERIC_PRECISION end
	// create table (..., col1 int(2) ...) -> the ddl is converted to create table (..., col1 int ...). Compatible to MySQL 8.0
	query := fmt.Sprintf(`
		SELECT
		    TABLE_SCHEMA, 
			TABLE_NAME,
			MD5(GROUP_CONCAT(
              CASE WHEN COLUMN_NAME NOT IN ('c_instance', 'c_schema', 'c_table') 
                THEN COLUMN_NAME 
              END 
            ORDER BY COLUMN_NAME ASC SEPARATOR ','
            )),
			MD5(GROUP_CONCAT(
              CASE WHEN COLUMN_NAME NOT IN ('c_instance', 'c_schema', 'c_table') 
                   THEN CONCAT_WS(':',         
                          COLUMN_NAME,
                          DATA_TYPE,
                          IS_NULLABLE,
                          CHARACTER_MAXIMUM_LENGTH,
                          CASE WHEN UPPER(DATA_TYPE) IN ('BIGINT', 'INT', 'MEDIUMINT', 'SMALLINT', 'TINYINT') 
                               THEN '0' ELSE NUMERIC_PRECISION END,
                          NUMERIC_SCALE,
                          DATETIME_PRECISION)
                   END 
              ORDER BY COLUMN_NAME ASC SEPARATOR ','
         )),
		 MAX(CASE WHEN COLUMN_NAME = 'c_instance' THEN 1 ELSE 0 END) AS DestHasSource,
         MAX(CASE WHEN COLUMN_NAME = 'c_schema'   THEN 1 ELSE 0 END) AS DestHasSchema,
         MAX(CASE WHEN COLUMN_NAME = 'c_table'    THEN 1 ELSE 0 END) AS DestHasTableName
		 FROM INFORMATION_SCHEMA.COLUMNS
		WHERE TABLE_SCHEMA in ('%s') 
		GROUP BY TABLE_SCHEMA, TABLE_NAME
		ORDER BY TABLE_SCHEMA, TABLE_NAME;
	`, strings.Join(dbInfo.DBs, "','"))

	//                 COLUMN_DEFAULT,

	// fmt.Printf("Query: %s \n", query)

	// 3. Define the database and table you want to query
	// databaseName := "orderdb_01"
	// tableName := "your_table_name"

	// 4. Prepare the SQL statement to prevent SQL injection
	stmt, err := db.Prepare(query)
	if err != nil {
		log.Fatalf("Failed to prepare statement: %v", err)
	}
	defer stmt.Close()

	// 5. Execute the query with the table names as parameters
	rows, err := stmt.Query()
	if err != nil {
		log.Fatalf("Failed to execute query: %v", err)
	}
	defer rows.Close()

	// 6. Iterate through the results
	for rows.Next() {
		var tableSchema, tableName, md5Columns, md5ColumnsWithTypes string
		var hasSource, hasSchema, hasTableName int
		if err := rows.Scan(&tableSchema, &tableName, &md5Columns, &md5ColumnsWithTypes, &hasSource, &hasSchema, &hasTableName); err != nil {
			log.Fatalf("Failed to scan row: %v", err)
		}

		// Create new TableInfo struct and append to slice
		newTableInfo := TableInfo{
			MD5Columns:          md5Columns,
			MD5ColumnsWithTypes: md5ColumnsWithTypes,
		}
		if tableType == "source" {
			newTableInfo.SrcTableInfo = []string{fmt.Sprintf("%s.%s.%s", dbInfo.Name, tableSchema, tableName)}
		} else {
			newTableInfo.DestTableInfo = []string{fmt.Sprintf("%s.%s.%s", dbInfo.Name, tableSchema, tableName)}
		}

		// Check if similar table structure exists
		found := false
		for i, existing := range *tableStructure {
			if existing.MD5Columns == newTableInfo.MD5Columns &&
				existing.MD5ColumnsWithTypes == newTableInfo.MD5ColumnsWithTypes {
				if tableType == "source" {
					existing.SrcTableInfo = append(existing.SrcTableInfo,
						fmt.Sprintf("%s.%s.%s", dbInfo.Name, tableSchema, tableName))
					(*tableStructure)[i] = existing
				} else {
					existing.DestTableInfo = append(existing.DestTableInfo,
						fmt.Sprintf("%s.%s.%s", dbInfo.Name, tableSchema, tableName))
					existing.DestHasSource = hasSource == 1
					existing.DestHasSchema = hasSchema == 1
					existing.DestHasTableName = hasTableName == 1
					(*tableStructure)[i] = existing
				}
				found = true
				break
			}
		}

		// If no match found, append new structure
		if !found {
			*tableStructure = append(*tableStructure, newTableInfo)
		}
	}

	if err := rows.Err(); err != nil {
		log.Fatalf("Error occurred during row iteration: %v", err)
	}

	return nil
}

type Config struct {
	SourceDB []DBConnInfo `yaml:"SourceDB"`
	DestDB   DBConnInfo   `yaml:"DestDB"`
	Template string       `yaml:"Template"`
	Output   string       `yaml:"output"`
	ErrorLog string       `yaml:"error_log"`
}

func readConfig(fileName string) (Config, error) {
	var config Config

	// Read the YAML file
	yamlFile, err := ioutil.ReadFile(fileName)
	if err != nil {
		return Config{}, fmt.Errorf("failed to read config file: %w", err)
	}

	// Unmarshal the YAML into the Config struct
	if err := yaml.Unmarshal(yamlFile, &config); err != nil {
		return Config{}, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Validate required fields
	if len(config.SourceDB) == 0 {
		return Config{}, fmt.Errorf("no source databases specified in config")
	}

	if config.DestDB.Host == "" {
		return Config{}, fmt.Errorf("destination database host not specified")
	}

	return config, nil
}

// calculateSampleSize determines how many items to sample from the input data
func calculateSampleSize(totalItems int) int {
	switch {
	case totalItems <= 100:
		return min(20, totalItems)
	case totalItems <= 1000:
		extraItems := int(0.01 * float64(totalItems-100))
		return min(20+extraItems, 50)
	default:
		return 50
	}
}

// sampleData takes a slice of data and returns a randomly sampled subset
func sampleData(data []string, sampleSize int) []string {
	if len(data) <= sampleSize {
		return data
	}

	half := sampleSize / 2
	sample := make([]string, 0, sampleSize)

	// Take first half
	sample = append(sample, data[:half]...)

	// Add middle placeholder
	middleCount := len(data) - sampleSize
	sample = append(sample, fmt.Sprintf("... other %d element ...", middleCount))

	// Take last half
	sample = append(sample, data[len(data)-half:]...)

	// Sort the sample before returning
	// sort.Strings(sample)
	return sample
}

func splitTables(tables []string) ([]string, []string) {
	tmpTables := []string{}
	// Convert table names from instanceName.DBName.Table format to DBName.Table
	// by removing the instanceName prefix
	dbList := []string{}
	tableList := []string{}
	for i := range tables {
		parts := strings.Split(tables[i], ".")
		if len(parts) == 3 {
			tmpTables = append(tmpTables, fmt.Sprintf("%s.%s", parts[1], parts[2]))
		}
		found := false
		for _, db := range dbList {
			if db == parts[1] {
				found = true
				break
			}
		}
		if !found {
			dbList = append(dbList, parts[1])
		}

		found = false
		for _, table := range tableList {
			if table == parts[2] {
				found = true
				break
			}
		}
		if !found {
			tableList = append(tableList, parts[2])
		}
	}

	return dbList, tableList
}

func fetchDumpingSourceData(srcInstance, srcSchema, srcTable string, hasSourceCol, hasSchemaCol, hasTableCol bool) string {
	if !hasSourceCol && !hasSchemaCol && !hasTableCol {
		return fmt.Sprintf("--tables-list '%s.%s'", srcSchema, srcTable)
	}
	var selectCols []string
	if hasSourceCol {
		selectCols = append(selectCols, fmt.Sprintf("'%s' as c_source", srcInstance))
	}
	if hasSchemaCol {
		selectCols = append(selectCols, fmt.Sprintf("'%s' as c_schema", srcSchema))
	}
	if hasTableCol {
		selectCols = append(selectCols, fmt.Sprintf("'%s' as c_table", srcTable))
	}
	return fmt.Sprintf("-S \"SELECT *, %s FROM %s.%s\"", strings.Join(selectCols, ", "), srcSchema, srcTable)
}
