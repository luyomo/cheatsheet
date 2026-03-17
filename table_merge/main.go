package main

import (
	"bytes"
	"context"
	"crypto/md5"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"
	"strconv"
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
	MaxID               int64
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
	logLevel   string
)

var rootCmd = &cobra.Command{
	Use:   "dm-toolkit",
	Short: "Toolkit to help DM",
	Run: func(cmd *cobra.Command, args []string) {
		// Exit if help flag is provided
		if cmd.Flag("help").Value.String() == "true" {
			os.Exit(0)
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
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "Log level (debug, info, warn, error)")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		slog.Error("rootCmd.Execute failed", "error", err)
		log.Fatal(err)
	}

	var err error

	err = initLog()
	if err != nil {
		slog.Error("failed to initialize logger", "error", err)
		log.Fatalf("Failed to initialize logger: %v", err)
		os.Exit(1)
	}

	var config Config
	if configFile != "" {
		slog.Info("reading config file", "configFile", configFile)
		config, err = readConfig(configFile)
		if err != nil {
			slog.Error("failed to read config file", "error", err, "configFile", configFile)
			log.Fatalf("Failed to read config file: %v", err)
		}
		slog.Debug("config loaded", "config", fmt.Sprintf("%#v", config))
	}

	if opsType == "" {
		slog.Warn("ops type not provided")
		fmt.Printf("Please provide ops type. \n")
		return
	}
	slog.Info("ops type", "opsType", opsType)

	tableStructure := []TableInfo{}

	/*
			Fetch source database table definitions and create a mapping where:
		    - Key: MD5 hash of consolidated column definitions
		    - Value: List of table names sharing the same column structure
	*/
	for _, sourceDB := range config.SourceDB {
		slog.Info("fetching source table definitions", "sourceDB", sourceDB.Name)
		err := fetch_table_def("source", &tableStructure, sourceDB)
		if err != nil {
			slog.Error("failed to fetch source table definitions", "error", err, "sourceDB", sourceDB.Name)
			fmt.Printf("Failed to fetch table definition: %v \n", err)
			return
		}
		slog.Debug("fetched source tables", "sourceDB", sourceDB.Name, "totalTables", len(tableStructure))
	}

	/*
			Similarly, fetch destination database table definitions and create a mapping where:
		    - Key: MD5 hash of consolidated column definitions
		    - Value: List of table names sharing the same column structure
	*/
	slog.Info("fetching destination table definitions", "destDB", config.DestDB.Name)
	err = fetch_table_def("dest", &tableStructure, config.DestDB)
	if err != nil {
		slog.Error("failed to fetch destination table definitions", "error", err, "destDB", config.DestDB.Name)
		fmt.Printf("Failed to fetch table definition: %v \n", err)
		return
	}
	slog.Debug("fetched destination tables", "destDB", config.DestDB.Name, "totalTables", len(tableStructure))

	slog.Debug("parsing template", "template", config.Template)
	tmpl := template.Must(template.New("dumpling").Parse(config.Template))

	// Open the output file for writing if specified.
	// Create file handlers for all the source db which will be used to output the dumpling command.
	mapWriter := make(map[string]*os.File)
	for _, db := range config.SourceDB {
		// Open output file for writing if specified
		var outputWriter *os.File
		if outputFile != "" {
			outPath := fmt.Sprintf("%s/%s.txt", outputFile, db.Name)
			slog.Info("creating output file", "path", outPath)
			var err error
			outputWriter, err = os.Create(outPath)
			if err != nil {
				slog.Error("failed to create output file", "error", err, "path", outPath)
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
		slog.Info("creating error output file", "path", outputErr)
		var err error
		errorWriter, err = os.Create(outputErr)
		if err != nil {
			slog.Error("failed to create error output file", "error", err, "path", outputErr)
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
						slog.Debug("matched same table name", "srcTable", srcTable, "destTable", destTable)
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
				slog.Debug("remaining many-to-many tables after name matching", "srcCount", len(tmpSrcTable), "destCount", len(tmpDestTable))
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
	slog.Info("table structure conversion completed", "finalTableCount", len(tableStructure))

	if opsType == "sourceAnalyze" {
		slog.Info("starting sourceAnalyze operation",
			"totalTableStructures", len(tableStructure),
			"description", "analyzing table mapping patterns between source and destination")
		// Pattern 01: one-to-one mapping
		slog.Debug("beginning pattern 01 scan", "pattern", "one-to-one")
		for idx, table := range tableStructure {
			if len(table.SrcTableInfo) == 1 && len(table.DestTableInfo) == 1 {
				slog.Info("pattern 01 detected",
					"index", idx,
					"md5Columns", table.MD5Columns,
					"md5ColumnsWithTypes", table.MD5ColumnsWithTypes,
					"srcTable", table.SrcTableInfo[0],
					"destTable", table.DestTableInfo[0])
				fmt.Printf("idx: %d, md5: %s, md5 with type: %s, Source table: %#v, Dest Table: %#v \n",
					idx, table.MD5Columns, table.MD5ColumnsWithTypes,
					table.SrcTableInfo[0], table.DestTableInfo[0])
			}
		}

		slog.Debug("pattern 01 scan complete")

		fmt.Printf("\n\n---------- Pattern 02: multiple-to-one pattern(No PK conflict) \n")
		slog.Debug("beginning pattern 02 scan", "pattern", "multiple-to-one (no PK conflict)")
		for idx, table := range tableStructure {
			if len(table.SrcTableInfo) > 1 && len(table.DestTableInfo) == 1 &&
				(!table.DestHasTableName && !table.DestHasSchema && !table.DestHasSource) {
				slog.Info("pattern 02 detected",
					"index", idx,
					"md5Columns", table.MD5Columns,
					"md5ColumnsWithTypes", table.MD5ColumnsWithTypes,
					"srcTableCount", len(table.SrcTableInfo),
					"firstSrcTable", table.SrcTableInfo[0],
					"destTable", table.DestTableInfo[0])
				fmt.Printf("idx: %d, md5: %s, md5 with type: %s, Source table(%d): %s ..., Dest Table: %s \n",
					idx, table.MD5Columns, table.MD5ColumnsWithTypes,
					len(table.SrcTableInfo), table.SrcTableInfo[0], table.DestTableInfo[0])
			}
		}
		slog.Debug("pattern 02 scan complete")

		fmt.Printf("\n\n---------- Pattern 03: multiple-to-one pattern(PK conflict) \n")
		slog.Debug("beginning pattern 03 scan", "pattern", "multiple-to-one (PK conflict)")
		for idx, table := range tableStructure {
			if len(table.SrcTableInfo) > 1 && len(table.DestTableInfo) == 1 &&
				(table.DestHasTableName || table.DestHasSchema || table.DestHasSource) {
				slog.Info("pattern 03 detected",
					"index", idx,
					"md5Columns", table.MD5Columns,
					"md5ColumnsWithTypes", table.MD5ColumnsWithTypes,
					"srcTableCount", len(table.SrcTableInfo),
					"firstSrcTable", table.SrcTableInfo[0],
					"destTable", table.DestTableInfo[0],
					"destHasTableName", table.DestHasTableName,
					"destHasSchema", table.DestHasSchema,
					"destHasSource", table.DestHasSource)
				fmt.Printf("idx: %d, md5: %s, md5 with type: %s, Source table(%d): %s ..., Dest Table: %s \n",
					idx, table.MD5Columns, table.MD5ColumnsWithTypes,
					len(table.SrcTableInfo), table.SrcTableInfo[0], table.DestTableInfo[0])
			}
		}
		slog.Debug("pattern 03 scan complete")

		fmt.Printf("\n\n---------- Pattern 04: multiple-to-multiple pattern \n")
		slog.Debug("beginning pattern 04 scan", "pattern", "multiple-to-multiple")
		for idx, table := range tableStructure {
			if len(table.SrcTableInfo) > 1 && len(table.DestTableInfo) > 1 {
				slog.Info("pattern 04 detected",
					"index", idx,
					"md5Columns", table.MD5Columns,
					"md5ColumnsWithTypes", table.MD5ColumnsWithTypes,
					"srcTableCount", len(table.SrcTableInfo),
					"srcTables", table.SrcTableInfo,
					"destTableCount", len(table.DestTableInfo),
					"destTables", table.DestTableInfo)
				fmt.Printf("idx: %d, md5: %s, md5 with type: %s, source table: %#v, # of source tables: %#v \n",
					idx, table.MD5Columns, table.MD5ColumnsWithTypes,
					table.SrcTableInfo, table.DestTableInfo)
			}
		}
		slog.Debug("pattern 04 scan complete")
		slog.Info("sourceAnalyze operation finished")
		return
	}

	if opsType == "generateDumpling" {
		slog.Info("starting generateDumpling operation",
			"totalTableStructures", len(tableStructure),
			"description", "generating dumpling commands for table mappings")
		for _, tableInfo := range tableStructure {
			// Case 1: One-to-one mapping
			if len(tableInfo.SrcTableInfo) == 1 && len(tableInfo.DestTableInfo) == 1 {
				srcTable := tableInfo.SrcTableInfo[0]
				srcParts := strings.Split(tableInfo.SrcTableInfo[0], ".")
				destParts := strings.Split(tableInfo.DestTableInfo[0], ".")

				sourceData := fetchDumpingSourceData(srcParts[0], srcParts[1], srcParts[2],
					tableInfo.DestHasSource, tableInfo.DestHasSchema, tableInfo.DestHasTableName)

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
					slog.Error("template execution failed",
						"case", "one-to-one",
						"srcTable", srcTable,
						"error", err)
					log.Printf("Error executing template: %v", err)
				} else {
					slog.Debug("dumpling command generated",
						"case", "one-to-one",
						"srcTable", srcTable,
						"destTable", tableInfo.DestTableInfo[0],
						"dbName", dbName)
					// fmt.Fprintf(mapWriter[dbName], "%s\n", buf.String())
				}
			}

			// Case 2: Many-to-many mapping with same table names and count
			if len(tableInfo.SrcTableInfo) > 1 && len(tableInfo.DestTableInfo) > 1 &&
				len(tableInfo.SrcTableInfo) == len(tableInfo.DestTableInfo) {
				slog.Debug("processing many-to-many mapping with same count",
					"srcCount", len(tableInfo.SrcTableInfo),
					"destCount", len(tableInfo.DestTableInfo))

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
								slog.Error("template execution failed",
									"case", "many-to-many",
									"srcTable", tableInfo.SrcTableInfo[i],
									"destTable", tableInfo.DestTableInfo[j],
									"error", err)
								log.Printf("Error executing template: %v", err)
							} else {
								slog.Debug("dumpling command generated",
									"case", "many-to-many",
									"srcTable", tableInfo.SrcTableInfo[i],
									"destTable", tableInfo.DestTableInfo[j],
									"dbName", dbName)
								fmt.Fprintf(mapWriter[dbName], "%s\n", buf.String())
							}
							break
						}
					}
				}
			}

			// Case 3: Many-to-one consolidation
			if len(tableInfo.SrcTableInfo) > 1 && len(tableInfo.DestTableInfo) == 1 {
				slog.Debug("processing many-to-one consolidation",
					"srcCount", len(tableInfo.SrcTableInfo),
					"destTable", tableInfo.DestTableInfo[0])

				destTable := tableInfo.DestTableInfo[0]
				destParts := strings.Split(destTable, ".")
				for idx, srcTable := range tableInfo.SrcTableInfo {
					srcParts := strings.Split(srcTable, ".")
					sourceData := fetchDumpingSourceData(srcParts[0], srcParts[1], srcParts[2],
						tableInfo.DestHasSource, tableInfo.DestHasSchema, tableInfo.DestHasTableName)

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
						slog.Error("template execution failed",
							"case", "many-to-one",
							"srcTable", srcTable,
							"destTable", destTable,
							"error", err)
						log.Printf("Error executing template: %v", err)
					} else {
						slog.Debug("dumpling command generated",
							"case", "many-to-one",
							"srcTable", srcTable,
							"destTable", destTable,
							"dbName", dbName,
							"consolidationIndex", idx+1)
						fmt.Fprintf(mapWriter[dbName], "%s\n", buf.String())
					}
				}
				// TODO: Implement consolidation logic
			}
		}
		slog.Info("generateDumpling operation finished")
	}

	// Generate the regex for table consolidations
	mapPatterns := make(map[string]string)
	if opsType == "generateSyncDiffconfig" || opsType == "generateDMConfig" {
		slog.Info("starting regex generation for table consolidations", "opsType", opsType, "totalTableStructures", len(tableStructure))
		for idx := range tableStructure {
			if len(tableStructure[idx].SrcTableInfo) > 2 {
				slog.Debug("processing table structure for regex generation",
					"index", idx,
					"srcTableCount", len(tableStructure[idx].SrcTableInfo),
					"destTableCount", len(tableStructure[idx].DestTableInfo),
					"md5Columns", tableStructure[idx].MD5Columns)

				// Get all source tables except the current one
				allSourceTables := make([]string, 0)
				for i := range tableStructure {
					if i != idx {
						allSourceTables = append(allSourceTables, tableStructure[i].SrcTableInfo...)
					}
				}
				slog.Debug("prepared exclusion list for regex generation",
					"currentIndex", idx,
					"exclusionCount", len(allSourceTables))

				regex, err := generateRegex(tableStructure[idx].SrcTableInfo, allSourceTables, mapPatterns)
				if err != nil {
					slog.Error("failed to generate regex for table consolidation",
						"error", err,
						"index", idx,
						"srcTables", tableStructure[idx].SrcTableInfo,
						"exclusionCount", len(allSourceTables))
				}

				if regex != nil {
					tableStructure[idx].SrcRegex = *regex
					slog.Debug("successfully generated regex",
						"index", idx,
						"regex", *regex,
						"srcTableCount", len(tableStructure[idx].SrcTableInfo))
				} else {
					slog.Warn("regex generation returned nil result",
						"index", idx,
						"srcTableCount", len(tableStructure[idx].SrcTableInfo))
				}
			}
		}
		slog.Info("completed regex generation for table consolidations", "processedCount", len(tableStructure))
	}

	// Fetch the max id from the target table for incremental diff operations
	slog.Info("starting max ID retrieval for incremental diff", "incrementalDiffTables", config.IncrementalDiffTables)
	err = SetMaxID4IncreDiff(config, tableStructure)
	if err != nil {
		slog.Error("failed to set max ID for incremental diff", "error", err)
		return
	}
	slog.Debug("completed max ID retrieval", "tableCount", len(tableStructure))

	// Debug: Log detailed table structure information
	// slog.Debug("table structure details",
	// 	"tableCount", len(tableStructure),
	// 	"tables", fmt.Sprintf("%+v", tableStructure))

	if opsType == "generateSyncDiffconfig" {
		slog.Info("starting sync diff config generation", "tableStructureCount", len(tableStructure))

		var syncDiffOutput *SyncDiffOutput
		summaryPath := "./output/summary.txt"
		if _, err := os.Stat(summaryPath); err == nil {
			slog.Info("found existing sync diff summary file", "path", summaryPath)
			syncDiffOutput, err = ParseSyncDiffOutput(summaryPath)
			if err != nil {
				slog.Error("failed to parse sync diff output", "error", err, "path", summaryPath)
				return
			}
			slog.Debug("parsed sync diff output", "inconsistentTableCount", len(syncDiffOutput.InconsistentTables))
		} else {
			slog.Debug("no existing sync diff summary file found", "path", summaryPath)
		}

		// Filter tableStructure to keep only those that failed in syncDiffOutput
		if syncDiffOutput != nil && len(syncDiffOutput.InconsistentTables) > 0 {
			slog.Info("filtering table structures based on inconsistent tables",
				"inconsistentCount", len(syncDiffOutput.InconsistentTables),
				"summaryPath", summaryPath)

			filtered := make([]TableInfo, 0, len(tableStructure))
			for _, ti := range tableStructure {
				for _, failed := range syncDiffOutput.InconsistentTables {
					// Match by destination table names (stored in ti.DestTableInfo)
					// Convert from instance.schemaName.tableName to schemaName.tableName format
					for _, dest := range ti.DestTableInfo {
						parts := strings.Split(dest, ".")
						if len(parts) == 3 {
							converted := fmt.Sprintf("%s.%s", parts[1], parts[2])
							if converted == failed.FullName {
								filtered = append(filtered, ti)
								slog.Debug("matched inconsistent table",
									"fullName", failed.FullName,
									"convertedName", converted,
									"md5Columns", ti.MD5Columns,
									"srcTableCount", len(ti.SrcTableInfo),
									"destTableCount", len(ti.DestTableInfo))
								break
							}
						} else {
							slog.Warn("unexpected destination table name format",
								"destTable", dest,
								"expectedFormat", "instance.schema.table")
						}
					}
				}
			}
			slog.Info("rendering sync diff config with filtered tables",
				"filteredCount", len(filtered),
				"originalCount", len(tableStructure),
				"inconsistentCount", len(syncDiffOutput.InconsistentTables))
			err = RenderSyncDiffConfig(&config, &filtered)
			if err != nil {
				slog.Error("failed to render sync diff config with filtered tables",
					"error", err,
					"filteredCount", len(filtered))
				return
			}
		} else {
			slog.Info("rendering sync diff config with all table structures",
				"totalCount", len(tableStructure),
				"hasInconsistentTables", syncDiffOutput != nil && len(syncDiffOutput.InconsistentTables) > 0)
			err = RenderSyncDiffConfig(&config, &tableStructure)
			if err != nil {
				slog.Error("failed to render sync diff config",
					"error", err,
					"totalCount", len(tableStructure))
				return
			}
		}
		slog.Info("completed sync diff config generation",
			"opsType", opsType,
			"configOutputPath", config.Output)
	}

	if opsType == "generateDMConfig" {
		slog.Info("starting DM config generation")
		err := RenderDMSourceConfig(&config)
		if err != nil {
			slog.Error("failed to render DM source config", "error", err)
			fmt.Printf("Error rendering DM source config: %v\n", err)
			return
		}
		slog.Debug("completed DM source config generation")

		err = RenderDMTaskConfig(&config, &tableStructure)
		if err != nil {
			slog.Error("failed to render DM task config", "error", err, "tableStructureCount", len(tableStructure))
			fmt.Printf("Error rendering DM task config: %v\n", err)
			return
		}
		slog.Info("completed DM config generation",
			"tableStructureCount", len(tableStructure),
			"sourceDBCount", len(config.SourceDB))
	}

	// Function already ends here; return is redundant and removed
}

type RuleResult struct {
	Rule string `json:"rule"`
}

func generateGeneralRegex(dataList []string, dataListShouldNotMatch []string) (*string, error) {
	var client *openai.Client
	if llmProduct == "" {
		slog.Warn("llmProduct not configured, returning placeholder regex", "dataList", dataList, "dataListShouldNotMatch", dataListShouldNotMatch)
		return &[]string{"---------- todo ----------"}[0], nil
	}

	// Log the LLM product selection for troubleshooting
	slog.Debug("configuring LLM client", "llmProduct", llmProduct)

	if llmProduct == "deepseek" {
		config := openai.DefaultConfig(os.Getenv("DEEPSEEK_API_KEY"))
		config.BaseURL = "https://api.deepseek.com/v1"
		client = openai.NewClientWithConfig(config)
		slog.Debug("deepseek client initialized", "baseURL", config.BaseURL)
	} else {
		client = openai.NewClient(os.Getenv("OPENAI_API_KEY"))
		slog.Debug("openai client initialized")
	}

	// Log the full data list for debugging pattern generation
	slog.Debug("generating regex pattern", "dataListCount", len(dataList), "shouldNotMatchCount", len(dataListShouldNotMatch), "dataList", strings.Join(dataList, ", "))

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

	// Log sampling details for troubleshooting
	slog.Debug("sampled data for pattern generation", "originalCount", len(dataList), "sampleSize", samplingNum, "sampledData", strings.Join(sampledData, ", "))

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
		slog.Debug("selected deepseek model", "model", model)
	} else {
		model = openai.GPT3Dot5Turbo
		slog.Debug("selected openai model", "model", model)
	}
	messages := []openai.ChatCompletionMessage{system, user}
	const maxRounds = 5
	for round := 1; round <= maxRounds; round++ {
		slog.Debug("starting LLM conversation round", "round", round, "maxRounds", maxRounds)

		if round == 4 {
			// Log full conversation history on round 4 for deep debugging
			slog.Debug("dumping conversation history for debugging", "round", round)
			for _, message := range messages {
				slog.Debug("conversation message", "role", message.Role, "content", message.Content)
			}
		}

		resp, err := client.CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{
			Model:       model,
			Messages:    messages,
			Temperature: 0.7,
			Tools:       tools,
		})
		if err != nil {
			slog.Error("LLM chat completion failed", "round", round, "error", err, "model", model)
			log.Fatalf("ChatCompletion error (round %d): %v", round, err)
		}

		assistant := resp.Choices[0].Message
		slog.Debug("received LLM response", "round", round, "hasToolCalls", len(assistant.ToolCalls) > 0)

		if len(assistant.ToolCalls) > 0 {
			// Add the assistant message with tool_calls to history
			messages = append(messages, assistant)

			for _, tc := range assistant.ToolCalls {
				if tc.Function.Name != "rule_is_valid" {
					slog.Debug("skipping non-rule_is_valid tool call", "toolName", tc.Function.Name)
					continue
				}

				// Parse arguments
				var args RuleResult
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
					// If parsing fails, give the model a helpful error signal
					slog.Error("failed to parse rule_is_valid arguments", "error", err, "arguments", tc.Function.Arguments)
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
				slog.Debug("validating generated rule", "rule", args.Rule, "dataListCount", len(dataList), "shouldNotMatchCount", len(dataListShouldNotMatch))
				toolContent := rule_is_valid(args.Rule, dataList, dataListShouldNotMatch)
				slog.Debug("rule validation completed", "rule", args.Rule, "valid", toolContent.Valid, "missedMatches", len(toolContent.MissedMatches), "falsePositives", len(toolContent.FalsePositives))

				if toolContent.Valid {
					slog.Info("successfully generated valid regex pattern", "rule", toolContent.Rule, "round", round)
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
			slog.Debug("validating rule from assistant content", "rule", rule)
			result := rule_is_valid(rule, dataList, dataListShouldNotMatch)
			if result.Valid {
				slog.Info("successfully generated valid regex pattern from content", "rule", result.Rule, "round", round)
				return &rule, nil
			}
			slog.Debug("rule validation failed", "rule", rule, "missedMatches", len(result.MissedMatches), "falsePositives", len(result.FalsePositives))
		}
	}

	slog.Error("failed to generate regex after max rounds", "maxRounds", maxRounds, "dataListCount", len(dataList), "shouldNotMatchCount", len(dataListShouldNotMatch))
	fmt.Println("********** Failed: Stopped after max rounds without a final answer. ")

	return nil, fmt.Errorf("failed to generate regex")
}

/*
 * This regex generation is used to detect the tables that are in the same structure for sync_diff_inspector which
 * only allow one routes.rule to compare the data between source tables and destination table. The only one regex is required
 * to conver all the source tables while it should not match any other tables.
 */
func generateRegex(tables []string, tablesShouldNotMatch []string, mapPatterns map[string]string) (*string, error) {
	slog.Debug("starting regex generation",
		"inputTableCount", len(tables),
		"exclusionCount", len(tablesShouldNotMatch),
		"cacheSize", len(mapPatterns))

	var err error

	dbList, tableList := splitTables(tables)
	_, tableListExclude := splitTables(tablesShouldNotMatch)

	slog.Debug("split tables into components",
		"dbList", dbList,
		"tableList", tableList,
		"tableListExclude", tableListExclude)

	var dbRegex *string
	if len(dbList) == 1 {
		dbRegex = &dbList[0]
		slog.Debug("using single db name as regex", "db", *dbRegex)
	} else {
		// Calculate MD5 of dbList and lookup in mapPatterns
		dbListStr := strings.Join(dbList, ",")
		dbListMD5 := fmt.Sprintf("%x", md5.Sum([]byte(dbListStr)))
		slog.Debug("calculated db list MD5", "md5", dbListMD5, "dbListStr", dbListStr)

		if cachedRegex, ok := mapPatterns[dbListMD5]; ok {
			dbRegex = &cachedRegex
			slog.Debug("found cached db regex", "md5", dbListMD5, "regex", *dbRegex)
		} else {
			slog.Debug("generating new db regex", "dbList", dbList)
			dbRegex, err = generateGeneralRegex(dbList, nil)
			if err != nil {
				slog.Error("failed to generate db regex", "error", err, "dbList", dbList)
				return nil, err
			}
			if dbRegex != nil {
				mapPatterns[dbListMD5] = *dbRegex
				slog.Debug("cached newly generated db regex", "md5", dbListMD5, "regex", *dbRegex)
			}
		}
	}

	var tableRegex *string
	if len(tableList) == 1 {
		tableRegex = &tableList[0]
		slog.Debug("using single table name as regex", "table", *tableRegex)
	} else {
		tableListMD5 := fmt.Sprintf("%x", md5.Sum([]byte(strings.Join(tableList, ","))))
		slog.Debug("calculated table list MD5", "md5", tableListMD5)

		if cachedRegex, ok := mapPatterns[tableListMD5]; ok {
			tableRegex = &cachedRegex
			slog.Debug("found cached table regex", "md5", tableListMD5, "regex", *tableRegex)
		} else {
			slog.Debug("generating new table regex", "tableList", tableList, "excludeList", tableListExclude)
			tableRegex, err = generateGeneralRegex(tableList, tableListExclude)
			if err != nil {
				slog.Error("failed to generate table regex", "error", err, "tableList", tableList, "excludeList", tableListExclude)
				return nil, err
			}
			if tableRegex != nil {
				mapPatterns[tableListMD5] = *tableRegex
				slog.Debug("cached newly generated table regex", "md5", tableListMD5, "regex", *tableRegex)
			}
		}
	}

	if dbRegex == nil || tableRegex == nil {
		emptyStr := ""
		slog.Warn("regex generation returned nil component", "dbRegex", dbRegex != nil, "tableRegex", tableRegex != nil)
		return &emptyStr, nil
	}
	regex := fmt.Sprintf("%s.%s", *dbRegex, *tableRegex)
	slog.Debug("final regex assembled", "regex", regex)

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
	slog.Debug("starting rule validation",
		"pattern", pattern,
		"tablesToMatch", len(tables),
		"tablesToExclude", len(tablesShouldNotMatch))

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
		slog.Error("failed to insert rule into trie selector", "error", err, "pattern", pattern, "schema", schema)
		result.Valid = false
		result.Error = fmt.Sprintf("Failed to insert rule: %v", err)
		return result
	}
	slog.Debug("rule inserted successfully", "pattern", pattern, "schema", schema)

	// Test all the tables which should match the rule
	for _, table := range tables {
		rules := ts.Match(schema, table)
		if rules == nil {
			result.MissedMatches = append(result.MissedMatches, table)
			slog.Debug("table failed to match rule", "table", table, "pattern", pattern)
			result.Valid = false
		} else {
			slog.Debug("table matched rule", "table", table, "pattern", pattern)
		}
	}

	// Test all the tables which should not match the rule
	for _, table := range tablesShouldNotMatch {
		rules := ts.Match(schema, table)
		if rules != nil {
			result.FalsePositives = append(result.FalsePositives, table)
			slog.Debug("table incorrectly matched rule (false positive)", "table", table, "pattern", pattern, "matchedRules", rules)
			result.Valid = false
		} else {
			slog.Debug("table correctly excluded", "table", table, "pattern", pattern)
		}
	}

	if len(result.MissedMatches) > 0 {
		result.Valid = false
		result.Error = "The validator reports these missed matches (see tool message). You must refine the rule so that all previously provided names match."
		slog.Debug("validation failed due to missed matches", "missedCount", len(result.MissedMatches), "missedTables", result.MissedMatches)
	}

	if len(result.FalsePositives) > 0 {
		result.Valid = false
		result.Error = fmt.Sprintf("%s \nThe validator reports false positives (see tool message). You must refine the rule so that all of names are excluded.", result.Error)
		slog.Debug("validation failed due to false positives", "falsePositiveCount", len(result.FalsePositives), "falsePositiveTables", result.FalsePositives)
	}

	slog.Debug("rule validation completed", "valid", result.Valid, "pattern", pattern)
	return result
}
func fetch_table_def(tableType string, tableStructure *[]TableInfo, dbInfo DBConnInfo) error {
	// The Data Source Name (DSN) string
	// Format: "user:password@tcp(host:port)/database?param=value"
	// Replace with your actual database credentials
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", dbInfo.User, dbInfo.Password, dbInfo.Host, dbInfo.Port, dbInfo.DBs[0])
	slog.Debug("building DSN for fetch_table_def", "tableType", tableType, "dbName", dbInfo.Name, "host", dbInfo.Host, "port", dbInfo.Port, "dbCount", len(dbInfo.DBs))

	// 1. Open a database handle
	// This does not yet establish a connection, but it prepares the database object.
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		slog.Error("failed to open database connection", "error", err, "tableType", tableType, "dbName", dbInfo.Name, "dsn", dsn)
		return fmt.Errorf("open mysql: %w", err)
	}
	// Ensure the connection is closed when the main function exits.
	defer db.Close()

	// 2. Ping the database to verify the connection
	// This performs a real check to see if the database is reachable.
	if err := db.Ping(); err != nil {
		slog.Error("failed to ping database", "error", err, "tableType", tableType, "dbName", dbInfo.Name)
		return fmt.Errorf("ping mysql: %w", err)
	}
	slog.Debug("successfully connected to database", "tableType", tableType, "dbName", dbInfo.Name)

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
	slog.Debug("generated INFORMATION_SCHEMA query", "tableType", tableType, "dbName", dbInfo.Name, "dbList", strings.Join(dbInfo.DBs, ","))

	//                 COLUMN_DEFAULT,

	// fmt.Printf("Query: %s \n", query)

	// 3. Define the database and table you want to query
	// databaseName := "orderdb_01"
	// tableName := "your_table_name"

	// 4. Prepare the SQL statement to prevent SQL injection
	stmt, err := db.Prepare(query)
	if err != nil {
		slog.Error("failed to prepare statement", "error", err, "tableType", tableType, "dbName", dbInfo.Name)
		return fmt.Errorf("prepare query: %w", err)
	}
	defer stmt.Close()

	// 5. Execute the query with the table names as parameters
	rows, err := stmt.Query()
	if err != nil {
		slog.Error("failed to execute query", "error", err, "tableType", tableType, "dbName", dbInfo.Name)
		return fmt.Errorf("execute query: %w", err)
	}
	defer rows.Close()

	// 6. Iterate through the results
	rowCount := 0
	for rows.Next() {
		var tableSchema, tableName, md5Columns, md5ColumnsWithTypes string
		var hasSource, hasSchema, hasTableName int
		if err := rows.Scan(&tableSchema, &tableName, &md5Columns, &md5ColumnsWithTypes, &hasSource, &hasSchema, &hasTableName); err != nil {
			slog.Error("failed to scan row", "error", err, "tableType", tableType, "dbName", dbInfo.Name)
			return fmt.Errorf("scan row: %w", err)
		}
		rowCount++
		slog.Debug("scanned table metadata", "tableType", tableType, "dbName", dbInfo.Name, "schema", tableSchema, "table", tableName, "md5Columns", md5Columns, "md5ColumnsWithTypes", md5ColumnsWithTypes)

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
				slog.Debug("merged table into existing structure", "tableType", tableType, "schema", tableSchema, "table", tableName, "md5Columns", md5Columns)
				break
			}
		}

		// If no match found, append new structure
		if !found {
			*tableStructure = append(*tableStructure, newTableInfo)
			slog.Debug("appended new table structure", "tableType", tableType, "schema", tableSchema, "table", tableName, "md5Columns", md5Columns)
		}
	}

	if err := rows.Err(); err != nil {
		slog.Error("error occurred during row iteration", "error", err, "tableType", tableType, "dbName", dbInfo.Name)
		return fmt.Errorf("rows iteration: %w", err)
	}

	slog.Info("completed fetch_table_def", "tableType", tableType, "dbName", dbInfo.Name, "rowCount", rowCount, "totalStructures", len(*tableStructure))
	return nil
}

type Config struct {
	SourceDB              []DBConnInfo `yaml:"SourceDB"`
	DestDB                DBConnInfo   `yaml:"DestDB"`
	Template              string       `yaml:"Template"`
	Output                string       `yaml:"Output"`
	ErrorLog              string       `yaml:"error_log"`
	IncrementalDiffTables []string     `yaml:"IncrementalDiffTables"`
}

func readConfig(fileName string) (Config, error) {
	var config Config

	// Read the YAML file
	yamlFile, err := os.ReadFile(fileName)
	if err != nil {
		slog.Error("failed to read config file", "fileName", fileName, "error", err)
		return Config{}, fmt.Errorf("failed to read config file: %w", err)
	}

	// Unmarshal the YAML into the Config struct
	if err := yaml.Unmarshal(yamlFile, &config); err != nil {
		slog.Error("failed to parse config file", "fileName", fileName, "error", err)
		return Config{}, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Validate required fields
	if len(config.SourceDB) == 0 {
		slog.Error("no source databases specified in config", "fileName", fileName)
		return Config{}, fmt.Errorf("no source databases specified in config")
	}

	if config.DestDB.Host == "" {
		slog.Error("destination database host not specified in config", "fileName", fileName)
		return Config{}, fmt.Errorf("destination database host not specified")
	}

	slog.Debug("successfully read and validated config", "fileName", fileName, "sourceDBCount", len(config.SourceDB), "destDBHost", config.DestDB.Host)
	return config, nil
}

// calculateSampleSize determines how many items to sample from the input data
func calculateSampleSize(totalItems int) int {
	switch {
	case totalItems <= 100:
		size := min(20, totalItems)
		slog.Debug("calculated sample size for small dataset", "totalItems", totalItems, "sampleSize", size)
		return size
	case totalItems <= 1000:
		extraItems := int(0.01 * float64(totalItems-100))
		size := min(20+extraItems, 50)
		slog.Debug("calculated sample size for medium dataset", "totalItems", totalItems, "sampleSize", size, "extraItems", extraItems)
		return size
	default:
		slog.Debug("calculated sample size for large dataset", "totalItems", totalItems, "sampleSize", 50)
		return 50
	}
}

// sampleData takes a slice of data and returns a randomly sampled subset
func sampleData(data []string, sampleSize int) []string {
	if len(data) <= sampleSize {
		slog.Debug("dataset smaller than or equal to sample size, returning full dataset", "dataSize", len(data), "sampleSize", sampleSize)
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

	slog.Debug("generated sample data", "originalSize", len(data), "sampleSize", sampleSize, "firstHalfSize", half, "lastHalfSize", half, "middlePlaceholder", middleCount)
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

	slog.Debug("split tables into components", "inputTableCount", len(tables), "dbCount", len(dbList), "tableCount", len(tableList), "dbList", dbList, "tableList", tableList)
	return dbList, tableList
}

func fetchDumpingSourceData(srcInstance, srcSchema, srcTable string, hasSourceCol, hasSchemaCol, hasTableCol bool) string {
	slog.Debug("generating dumping source data", "srcInstance", srcInstance, "srcSchema", srcSchema, "srcTable", srcTable, "hasSourceCol", hasSourceCol, "hasSchemaCol", hasSchemaCol, "hasTableCol", hasTableCol)
	if !hasSourceCol && !hasSchemaCol && !hasTableCol {
		result := fmt.Sprintf("--tables-list '%s.%s'", srcSchema, srcTable)
		slog.Debug("no metadata columns needed, using simple table list", "result", result)
		return result
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
	result := fmt.Sprintf("-S \"SELECT *, %s FROM %s.%s\"", strings.Join(selectCols, ", "), srcSchema, srcTable)
	slog.Debug("metadata columns needed, generated SELECT query", "selectCols", selectCols, "result", result)
	return result
}

func SetMaxID4IncreDiff(config Config, tableStructure []TableInfo) error {
	slog.Info("starting incremental diff max ID processing", "incrementalDiffTables", config.IncrementalDiffTables, "outputDir", config.Output)

	outputFile := fmt.Sprintf("%s/sync-diff-id.txt", config.Output)
	if _, err := os.Stat(outputFile); err == nil {
		// File exists, read it
		slog.Info("found existing max ID file, reading cached values", "outputFile", outputFile)
		content, err := os.ReadFile(outputFile)
		if err != nil {
			slog.Error("failed to read existing max ID file", "outputFile", outputFile, "error", err)
			return fmt.Errorf("failed to read file %s: %w", outputFile, err)
		}
		slog.Debug("read existing max ID file content", "outputFile", outputFile, "contentLength", len(content))
		// Parse each line in the content (format: schema.table:maxID)
		lines := strings.Split(string(content), "\n")
		parsedCount := 0
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			parts := strings.Split(line, ":")
			if len(parts) != 2 {
				slog.Warn("skipping malformed line in max ID file", "line", line, "outputFile", outputFile)
				continue
			}
			schemaTable := parts[0]
			maxID, err := strconv.ParseInt(parts[1], 10, 64)
			if err != nil {
				slog.Error("failed to parse maxID from line", "line", line, "schemaTable", schemaTable, "error", err)
				continue
			}
			parsedCount++

			// Find the matching table in tableStructure and set MaxID
			matched := false
			for i := range tableStructure {
				for _, destTable := range tableStructure[i].DestTableInfo {
					// Extract schema.table from destTable (instance.schema.table)
					destParts := strings.Split(destTable, ".")
					if len(destParts) == 3 {
						destSchemaTable := fmt.Sprintf("%s.%s", destParts[1], destParts[2])
						if destSchemaTable == schemaTable {
							tableStructure[i].MaxID = maxID
							matched = true
							slog.Debug("applied cached max ID to table structure", "schemaTable", schemaTable, "maxID", maxID, "destTable", destTable)
							break
						}
					}
				}
				if matched {
					break
				}
			}
			if !matched {
				slog.Warn("no matching table found for cached max ID", "schemaTable", schemaTable, "maxID", maxID)
			}
		}
		slog.Info("completed loading cached max IDs", "parsedCount", parsedCount, "totalLines", len(lines))
		return nil
	} else if errors.Is(err, os.ErrNotExist) {
		// File does not exist, create it
		slog.Info("no existing max ID file found, will query databases for max IDs", "outputFile", outputFile)
	} else {
		// Some other error
		slog.Error("error checking max ID file existence", "outputFile", outputFile, "error", err)
		return fmt.Errorf("error checking file %s: %w", outputFile, err)
	}
	mapMaxIDs := make(map[string]int64)

	// Loop through tableStructure and find items whose target table is in IncrementalDiffTables
	tablesProcessed := 0
	queriesExecuted := 0
	for i := range tableStructure {
		tableInfo := &tableStructure[i]
		for _, destTable := range tableInfo.DestTableInfo {
			// Extract SchemaName.TableName from destTable (targetName.SchemaName.TableName)
			destParts := strings.Split(destTable, ".")
			if len(destParts) != 3 {
				slog.Warn("skipping destTable with unexpected format", "destTable", destTable)
				continue
			}
			destSchemaTable := fmt.Sprintf("%s.%s", destParts[1], destParts[2])

			for _, incTable := range config.IncrementalDiffTables {
				if destSchemaTable == incTable {
					tablesProcessed++
					slog.Debug("processing incremental diff table", "destSchemaTable", destSchemaTable, "incTable", incTable)

					for _, srcTable := range tableInfo.SrcTableInfo {
						parts := strings.Split(srcTable, ".")
						if len(parts) == 3 {
							instance := parts[0]
							schemaTable := fmt.Sprintf("%s.%s", parts[1], parts[2])
							slog.Debug("processing source table for max ID", "instance", instance, "schemaTable", schemaTable)

							// Find the DB config for this instance
							var dbConfig *DBConnInfo
							for _, srcDB := range config.SourceDB {
								if srcDB.Name == instance {
									dbConfig = &srcDB
									break
								}
							}
							if dbConfig != nil {
								// Prepare DSN and open DB connection
								dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", dbConfig.User, dbConfig.Password, dbConfig.Host, dbConfig.Port, dbConfig.DBs[0])
								db, err := sql.Open("mysql", dsn)
								if err != nil {
									slog.Error("failed to open DB connection for max ID query", "instance", instance, "schemaTable", schemaTable, "error", err)
									continue
								}
								// Note: defer in loop is generally discouraged, but acceptable here due to limited iteration count
								defer db.Close()

								if err := db.Ping(); err != nil {
									slog.Error("failed to ping DB for max ID query", "instance", instance, "schemaTable", schemaTable, "error", err)
									continue
								}

								// Run the query: select max(id) from schemaTable
								var maxID sql.NullInt64
								query := fmt.Sprintf("SELECT MAX(id) FROM %s", schemaTable)
								err = db.QueryRow(query).Scan(&maxID)
								queriesExecuted++
								if err != nil {
									slog.Error("failed to get max id from table", "instance", instance, "schemaTable", schemaTable, "query", query, "error", err)
									continue
								}
								if maxID.Valid {
									slog.Debug("retrieved max ID from source table", "instance", instance, "schemaTable", schemaTable, "maxID", maxID.Int64)
									if tableInfo.MaxID < maxID.Int64 {
										tableInfo.MaxID = maxID.Int64
										mapMaxIDs[destSchemaTable] = maxID.Int64
										slog.Info("updated max ID for incremental diff table", "destSchemaTable", destSchemaTable, "maxID", maxID.Int64, "instance", instance, "schemaTable", schemaTable)
									}
								} else {
									slog.Warn("no rows found in source table for max ID query", "instance", instance, "schemaTable", schemaTable)
								}
							} else {
								slog.Warn("no DB config found for instance", "instance", instance, "destSchemaTable", destSchemaTable)
							}
						} else {
							slog.Warn("skipping source table with unexpected format", "srcTable", srcTable)
						}
					}
					break
				}
			}
		}
	}

	slog.Info("completed max ID queries", "tablesProcessed", tablesProcessed, "queriesExecuted", queriesExecuted, "maxIDsCollected", len(mapMaxIDs))

	// Write the mapMaxIDs to the output file
	file, err := os.Create(outputFile)
	if err != nil {
		slog.Error("failed to create max ID output file", "outputFile", outputFile, "error", err)
		return fmt.Errorf("failed to create file %s: %w", outputFile, err)
	}
	defer file.Close()

	writtenCount := 0
	for table, maxID := range mapMaxIDs {
		_, err := fmt.Fprintf(file, "%s:%d\n", table, maxID)
		if err != nil {
			slog.Error("failed to write max ID to file", "outputFile", outputFile, "table", table, "maxID", maxID, "error", err)
			return fmt.Errorf("failed to write to file %s: %w", outputFile, err)
		}
		writtenCount++
		slog.Debug("wrote max ID to file", "table", table, "maxID", maxID)
	}

	slog.Info("successfully wrote max IDs to file", "outputFile", outputFile, "writtenCount", writtenCount)
	return nil
}

func initLog() error {
	// Ensure log directory exists before opening log file
	if err := os.MkdirAll("log", 0755); err != nil {
		slog.Error("failed to create log directory", "error", err)
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	// Initialize slog logger and output to log/table_merge.log
	logFile, err := os.OpenFile("log/table_merge.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		slog.Error("failed to open log file", "error", err)
		return fmt.Errorf("failed to open log file: %w", err)
	}
	// Do not defer close here; the caller should manage the file lifecycle.

	var level slog.Level
	switch strings.ToLower(logLevel) {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}
	opts := &slog.HandlerOptions{Level: level, AddSource: true}
	logger := slog.New(slog.NewJSONHandler(logFile, opts))
	slog.SetDefault(logger)
	slog.Info("logger initialized", "logFile", "log/table_merge.log")
	return nil
}
