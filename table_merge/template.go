package main

import (
	"embed"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"text/template"
)

//go:embed templates/diff.tpl.toml templates/task.tpl.toml
var readmeFS embed.FS

type SyncDiffConfig struct {
	CheckThreadCount     int                    `yaml:"check-thread-count" json:"check_thread_count"`
	ExportFixSQL         bool                   `yaml:"export-fix-sql" json:"export_fix_sql"`
	CheckDataOnly        bool                   `yaml:"check-data-only" json:"check_data_only"`
	CheckStructOnly      bool                   `yaml:"check-struct-only" json:"check_struct_only"`
	SkipNonExistingTable bool                   `yaml:"skip-non-existing-table" json:"skip_non_existing_table"`
	DataSources          map[string]DataSource  `yaml:"data-sources" json:"data_sources"`
	Task                 TaskConfig             `yaml:"task" json:"task"`
	Routes               map[string]RouteRule   `yaml:"routes,omitempty" json:"routes,omitempty"`
	TableConfigs         map[string]TableConfig `yaml:"table-configs,omitempty" json:"table_configs,omitempty"`
}

// DataSource represents a database connection configuration
type DataSource struct {
	Host       string   `yaml:"host" json:"host"`
	Port       int      `yaml:"port" json:"port"`
	User       string   `yaml:"user" json:"user"`
	Password   string   `yaml:"password" json:"password"`
	TimeZone   string   `yaml:"time-zone,omitempty" json:"time_zone,omitempty"`
	Location   string   `yaml:"location,omitempty" json:"location,omitempty"`
	RouteRules []string `yaml:"route-rules,omitempty" json:"route_rules,omitempty"`
}

// TaskConfig represents the task configuration
type TaskConfig struct {
	OutputDir         string   `yaml:"output-dir" json:"output_dir"`
	SourceInstances   []string `yaml:"source-instances" json:"source_instances"`
	TargetInstance    string   `yaml:"target-instance" json:"target_instance"`
	TargetCheckTables []string `yaml:"target-check-tables,omitempty" json:"target_check_tables,omitempty"`
	TargetConfigs     []string `yaml:"target-configs,omitempty" json:"target_configs,omitempty"`
}

// RouteRule represents a routing rule for schema/table mapping
type RouteRule struct {
	SchemaPattern string `yaml:"schema-pattern" json:"schema_pattern"`
	TablePattern  string `yaml:"table-pattern" json:"table_pattern"`
	TargetSchema  string `yaml:"target-schema,omitempty" json:"target_schema,omitempty"`
	TargetTable   string `yaml:"target-table,omitempty" json:"target_table,omitempty"`
}

// TableConfig represents table-specific configurations
type TableConfig struct {
	TargetTables  []string `yaml:"target-tables" json:"target_tables"`
	IgnoreColumns []string `yaml:"ignore-columns" json:"ignore_columns"`
	Range         string   `yaml:"range,omitempty" json:"range,omitempty"`
}

func RenderSyncDiffConfig(config *Config, tableMapping *[]TableInfo) error {
	if config == nil {
		slog.Error("RenderSyncDiffConfig received nil config", "tableMappingLen", len(*tableMapping))
		return fmt.Errorf("config is nil")
	}
	if tableMapping == nil {
		slog.Error("RenderSyncDiffConfig received nil tableMapping", "configOutput", config.Output)
		return fmt.Errorf("tableMapping is nil")
	}

	slog.Info("starting RenderSyncDiffConfig", "output", config.Output, "sourceDBCount", len(config.SourceDB), "tableMappingCount", len(*tableMapping))

	syncDiffConfig := SyncDiffConfig{
		CheckThreadCount:     10,
		ExportFixSQL:         true,
		CheckDataOnly:        false,
		CheckStructOnly:      false,
		SkipNonExistingTable: false,
		DataSources:          make(map[string]DataSource),
		Task:                 TaskConfig{},
		Routes:               make(map[string]RouteRule),
		TableConfigs:         make(map[string]TableConfig),
	}

	// 01. Map config to DataSources including the routes for each source instance
	for _, ds := range config.SourceDB {
		slog.Debug("processing source DB", "dsName", ds.Name, "host", ds.Host, "port", ds.Port)

		// Collect route rules for this data source
		var routeRules []string
		for tiIdx, tableInfo := range *tableMapping {
			// Extract instance name from SrcTableInfo
			if len(tableInfo.DestTableInfo) == 0 {
				slog.Warn("tableInfo.DestTableInfo empty, skipping", "tableMappingIdx", tiIdx, "SrcTableInfo", tableInfo.SrcTableInfo)
				continue
			}
			destParts := strings.Split(tableInfo.DestTableInfo[0], ".")
			var ruleName string
			if len(destParts) > 2 {
				ruleName = "r_" + destParts[2]
			} else {
				slog.Warn("dest table info insufficient parts", "tableMappingIdx", tiIdx, "DestTableInfo", tableInfo.DestTableInfo[0], "SrcTableInfo", tableInfo.SrcTableInfo, "parts", destParts)
				continue
			}
			for _, src := range tableInfo.SrcTableInfo {
				parts := strings.Split(src, ".")
				if len(parts) > 0 && parts[0] == ds.Name {
					routeRules = append(routeRules, ruleName)
					slog.Debug("matched route rule", "dsName", ds.Name, "ruleName", ruleName, "src", src)
					break // at least one matched, append and stop checking further
				} else {
					slog.Debug("src table instance mismatch", "srcInstance", parts[0], "expectedInstance", ds.Name, "src", src)
				}
			}
		}
		slog.Info("built route rules for source DB", "dsName", ds.Name, "routeRules", routeRules)
		syncDiffConfig.DataSources[ds.Name] = DataSource{
			Host:       ds.Host,
			Port:       ds.Port,
			User:       ds.User,
			Password:   ds.Password,
			RouteRules: routeRules,
		}
	}

	// Map config.DestDB (single struct) to DataSources as well
	syncDiffConfig.DataSources[config.DestDB.Name] = DataSource{
		Host:     config.DestDB.Host,
		Port:     config.DestDB.Port,
		User:     config.DestDB.User,
		Password: config.DestDB.Password,
	}
	slog.Info("added destination DB to DataSources", "destName", config.DestDB.Name)

	// 02. Build routing rules from tableMapping
	for tiIdx, tableInfo := range *tableMapping {
		if len(tableInfo.DestTableInfo) == 0 {
			slog.Warn("tableInfo.DestTableInfo empty, skipping rule build", "tableMappingIdx", tiIdx, "SrcTableInfo", tableInfo.SrcTableInfo)
			continue
		}

		var schemaPattern, tablePattern string
		destParts := strings.Split(tableInfo.DestTableInfo[0], ".")
		if len(destParts) < 3 {
			slog.Warn("dest table info insufficient parts for rule", "tableMappingIdx", tiIdx, "DestTableInfo", tableInfo.DestTableInfo[0], "parts", destParts)
			continue
		}
		routeKey := "r_" + destParts[2]
		if tableInfo.SrcRegex == "" {
			if len(tableInfo.SrcTableInfo) == 0 {
				slog.Warn("tableInfo.SrcTableInfo empty and no SrcRegex", "tableMappingIdx", tiIdx, "DestTableInfo", tableInfo.DestTableInfo)
				continue
			}
			parts := strings.Split(tableInfo.SrcTableInfo[0], ".")
			if len(parts) > 2 {
				schemaPattern = parts[1]
				tablePattern = parts[2]
			} else {
				slog.Warn("src table info insufficient parts for rule", "tableMappingIdx", tiIdx, "SrcTableInfo", tableInfo.SrcTableInfo[0], "parts", parts)
				continue
			}
		} else {
			parts := strings.Split(tableInfo.SrcRegex, ".")
			if len(parts) > 1 {
				schemaPattern = parts[0]
				tablePattern = parts[1]
			} else {
				slog.Warn("SrcRegex invalid for rule", "tableMappingIdx", tiIdx, "SrcRegex", tableInfo.SrcRegex)
				continue
			}
		}
		syncDiffConfig.Routes[routeKey] = RouteRule{
			SchemaPattern: schemaPattern,
			TablePattern:  tablePattern,
			TargetSchema:  destParts[1],
			TargetTable:   destParts[2],
		}
		slog.Debug("built route rule", "routeKey", routeKey, "schemaPattern", schemaPattern, "tablePattern", tablePattern, "targetSchema", destParts[1], "targetTable", destParts[2])
	}

	// 03. Build source-instances list
	sourceInstances := make([]string, 0, len(config.SourceDB))
	for _, ds := range config.SourceDB {
		sourceInstances = append(sourceInstances, ds.Name)
	}
	slog.Info("built source instances list", "sourceInstances", sourceInstances)

	// 04. Build target-check-tables list
	targetCheckTables := make([]string, 0, len(*tableMapping))
	for tiIdx, tableInfo := range *tableMapping {
		if len(tableInfo.DestTableInfo) > 0 {
			parts := strings.SplitN(tableInfo.DestTableInfo[0], ".", 3)
			if len(parts) == 3 {
				targetCheckTables = append(targetCheckTables, parts[1]+"."+parts[2])
			} else {
				slog.Warn("unexpected DestTableInfo format", "tableMappingIdx", tiIdx, "DestTableInfo", tableInfo.DestTableInfo[0], "parts", parts)
				targetCheckTables = append(targetCheckTables, tableInfo.DestTableInfo[0])
			}
		}
	}
	slog.Info("built target check tables", "targetCheckTables", targetCheckTables)

	// 05. Set Task field
	syncDiffConfig.Task = TaskConfig{
		OutputDir:         "./output",
		SourceInstances:   sourceInstances,
		TargetInstance:    config.DestDB.Name,
		TargetCheckTables: targetCheckTables,
	}
	slog.Debug("set Task field", "outputDir", syncDiffConfig.Task.OutputDir, "targetInstance", syncDiffConfig.Task.TargetInstance)

	// 06. Build mapExcludeColumns
	var mapExcludeColumns = make(map[string][]string)
	for tiIdx, tbl := range *tableMapping {
		if len(tbl.DestTableInfo) == 0 {
			slog.Warn("tbl.DestTableInfo empty, skipping exclude build", "tableMappingIdx", tiIdx, "SrcTableInfo", tbl.SrcTableInfo)
			continue
		}
		destParts := strings.Split(tbl.DestTableInfo[0], ".")
		if len(destParts) < 3 {
			slog.Warn("dest table info insufficient parts for exclude", "tableMappingIdx", tiIdx, "DestTableInfo", tbl.DestTableInfo[0], "parts", destParts)
			continue
		}
		schema := destParts[1]
		table := destParts[2]

		parts := []string{}
		if tbl.DestHasSource {
			parts = append(parts, "c_instance")
		}
		if tbl.DestHasSchema {
			parts = append(parts, "c_schema")
		}
		if tbl.DestHasTableName {
			parts = append(parts, "c_table")
		}
		if len(parts) == 0 {
			slog.Debug("no exclude flags set", "tableMappingIdx", tiIdx, "schema", schema, "table", table)
			continue
		}
		key := strings.Join(parts, ",")
		mapExcludeColumns[key] = append(mapExcludeColumns[key], schema+"."+table)
		slog.Debug("added exclude entry", "key", key, "schemaTable", schema+"."+table)
	}
	slog.Info("built mapExcludeColumns", "mapExcludeColumns", mapExcludeColumns)

	// 07. Loop mapExcludeColumns and prepare the TableConfigs
	idx := 0
	for key, tables := range mapExcludeColumns {
		cfgKey := fmt.Sprintf("cfg_%d", idx)
		syncDiffConfig.TableConfigs[cfgKey] = TableConfig{
			TargetTables:  tables,
			IgnoreColumns: strings.Split(key, ","),
		}
		syncDiffConfig.Task.TargetConfigs = append(syncDiffConfig.Task.TargetConfigs, cfgKey)
		slog.Debug("added TableConfig for exclude", "cfgKey", cfgKey, "tables", tables, "ignoreCols", strings.Split(key, ","))
		idx++
	}

	// 08. Loop all tableInfos again and prepare TableConfigs for items with MaxID > 0
	for tiIdx, tbl := range *tableMapping {
		if tbl.MaxID <= 0 {
			continue
		}
		if len(tbl.DestTableInfo) == 0 {
			slog.Warn("tbl.DestTableInfo empty for MaxID entry", "tableMappingIdx", tiIdx, "MaxID", tbl.MaxID)
			continue
		}
		destParts := strings.Split(tbl.DestTableInfo[0], ".")
		if len(destParts) < 3 {
			slog.Warn("dest table info insufficient parts for MaxID entry", "tableMappingIdx", tiIdx, "DestTableInfo", tbl.DestTableInfo[0], "MaxID", tbl.MaxID, "parts", destParts)
			continue
		}
		schema := destParts[1]
		table := destParts[2]

		cfgKey := fmt.Sprintf("range_cfg_%s_%s", schema, table)
		syncDiffConfig.TableConfigs[cfgKey] = TableConfig{
			TargetTables: []string{schema + "." + table},
			Range:        fmt.Sprintf("id <= %d", tbl.MaxID),
		}
		syncDiffConfig.Task.TargetConfigs = append(syncDiffConfig.Task.TargetConfigs, cfgKey)
		slog.Debug("added range TableConfig", "cfgKey", cfgKey, "schema", schema, "table", table, "range", fmt.Sprintf("id <= %d", tbl.MaxID))
	}

	// 09. Write output file
	outputPath := config.Output
	if !strings.HasSuffix(outputPath, "/") {
		outputPath += "/"
	}
	outFileName := outputPath + "sync-diff.toml"
	outFile, err := os.Create(outFileName)
	if err != nil {
		slog.Error("failed to create output file", "file", outFileName, "error", err)
		return fmt.Errorf("failed to create output file %s: %w", outFileName, err)
	}
	defer outFile.Close()

	tmplBytes, err := readmeFS.ReadFile("templates/diff.tpl.toml")
	if err != nil {
		slog.Error("failed to read template file", "template", "templates/diff.tpl.toml", "error", err)
		return fmt.Errorf("failed to read template file: %w", err)
	}

	tmpl, err := template.New("diff").Parse(string(tmplBytes))
	if err != nil {
		slog.Error("failed to parse template", "error", err)
		return fmt.Errorf("failed to parse template: %w", err)
	}

	if err := tmpl.Execute(outFile, syncDiffConfig); err != nil {
		slog.Error("failed to execute template", "file", outFileName, "error", err)
		return fmt.Errorf("failed to execute template: %w", err)
	}

	slog.Info("successfully rendered sync-diff.toml", "file", outFileName)
	return nil
}

func RenderDMSourceConfig(config *Config) error {
	if config == nil {
		slog.Error("RenderDMSourceConfig received nil config")
		return fmt.Errorf("config is nil")
	}

	// Define the data structure for the template
	type DMTemplateData struct {
		SourceID        string
		ServerID        int
		Flavor          string
		EnableGTID      bool
		EnableRelay     bool
		RelayBinlogName string
		RelayBinlogGTID string
		Host            string
		User            string
		Password        string
		Port            int
		EnableChecker   bool
	}

	// Template string
	const dmTemplate = `source-id: "{{.SourceID}}"
server-id: {{.ServerID}}
flavor: "{{with .Flavor}}{{.}}{{else}}mysql{{end}}"
enable-gtid: {{.EnableGTID}}

# Relay log settings
enable-relay: {{.EnableRelay}}
relay-binlog-name: "{{.RelayBinlogName}}"
relay-binlog-gtid: "{{.RelayBinlogGTID}}"

# Connection details
from:
  host: "{{.Host}}"
  user: "{{.User}}"
  password: "{{.Password}}"
  port: {{.Port}}

# Pre-migration checks
checker:
  check-enable: {{.EnableChecker}}
`

	slog.Info("starting RenderDMSourceConfig", "output", config.Output, "sourceDBCount", len(config.SourceDB))

	// Loop over each source DB and generate a config file
	for i, db := range config.SourceDB {
		slog.Debug("processing source DB", "dbName", db.Name, "host", db.Host, "port", db.Port)

		data := DMTemplateData{
			SourceID:      fmt.Sprintf("mysql-sourcedb-%d", 10000+i),
			ServerID:      10000 + i,
			Flavor:        "mysql",
			EnableGTID:    false,
			EnableRelay:   false,
			EnableChecker: true,
			Host:          db.Host,
			Port:          db.Port,
			User:          db.User,
			Password:      db.Password,
		}

		// Parse and execute the template
		tmpl, err := template.New("dm").Parse(dmTemplate)
		if err != nil {
			slog.Error("failed to parse DM template", "error", err)
			return fmt.Errorf("failed to parse DM template: %w", err)
		}

		// Create output file
		outputPath := config.Output
		if !strings.HasSuffix(outputPath, "/") {
			outputPath += "/"
		}
		outFileName := fmt.Sprintf("%sdm-source-%s.yaml", outputPath, db.Name)
		slog.Debug("creating DM source config file", "file", outFileName)
		outFile, err := os.Create(outFileName)
		if err != nil {
			slog.Error("failed to create output file", "file", outFileName, "error", err)
			return fmt.Errorf("failed to create output file %s: %w", outFileName, err)
		}
		defer outFile.Close()

		if err := tmpl.Execute(outFile, data); err != nil {
			slog.Error("failed to execute template", "file", outFileName, "error", err)
			return fmt.Errorf("failed to execute template for %s: %w", db.Name, err)
		}

		slog.Info("successfully rendered DM source config", "file", outFileName)
	}

	slog.Info("completed RenderDMSourceConfig", "output", config.Output, "filesGenerated", len(config.SourceDB))
	return nil
}

func RenderDMTaskConfig(config *Config, tableMapping *[]TableInfo) error {
	if config == nil {
		slog.Error("RenderDMTaskConfig received nil config")
		return fmt.Errorf("config is nil")
	}
	if tableMapping == nil {
		slog.Error("RenderDMTaskConfig received nil tableMapping", "configOutput", config.Output)
		return fmt.Errorf("tableMapping is nil")
	}

	slog.Info("starting RenderDMTaskConfig", "output", config.Output, "sourceDBCount", len(config.SourceDB), "tableMappingCount", len(*tableMapping))

	// Define the data structure for the template
	type DMTaskTemplateData struct {
		Name       string
		TaskMode   string
		IsSharding bool
		MetaSchema string
		TargetDB   struct {
			Host     string
			Port     int
			User     string
			Password string
		}
		MySQLInstances []struct {
			SourceID     string
			InstanceName string
			RouteRules   []string
		}
		Validators struct {
			Mode        string
			WorkerCount int
			ErrorDelay  string
		}
		AllowList map[string][]string
		Routes    map[string]RouteRule
	}

	// Read the template file
	tmplBytes, err := readmeFS.ReadFile("templates/task.tpl.toml")
	if err != nil {
		slog.Error("failed to read template file", "template", "templates/task.tpl.toml", "error", err)
		return fmt.Errorf("failed to read template file: %w", err)
	}

	// Prepare template data
	data := DMTaskTemplateData{
		Name:       "dm-task",
		TaskMode:   "incremental",
		IsSharding: true,
		MetaSchema: "dm_meta",
		TargetDB: struct {
			Host     string
			Port     int
			User     string
			Password string
		}{
			Host:     config.DestDB.Host,
			Port:     config.DestDB.Port,
			User:     config.DestDB.User,
			Password: config.DestDB.Password,
		},
		Validators: struct {
			Mode        string
			WorkerCount int
			ErrorDelay  string
		}{
			Mode:        "full",
			WorkerCount: 4,
			ErrorDelay:  "30s",
		},
		AllowList: map[string][]string{},
		Routes:    make(map[string]RouteRule),
	}

	// Build MySQL instances
	for i, dbConnInfo := range config.SourceDB {
		slog.Debug("processing source DB", "dbName", dbConnInfo.Name, "host", dbConnInfo.Host, "port", dbConnInfo.Port)

		instance := struct {
			SourceID     string
			InstanceName string
			RouteRules   []string
		}{
			InstanceName: dbConnInfo.Name,
			SourceID:     fmt.Sprintf("mysql-sourcedb-%d", 10000+i),
		}

		allowList := []string{}

		// Collect route rules for this instance
		for tiIdx, tableInfo := range *tableMapping {
			// 01. Prepare the route name from the regrex and dest table name
			if tableInfo.SrcRegex == "" {
				if len(tableInfo.DestTableInfo) == 0 || len(tableInfo.SrcTableInfo) == 0 {
					slog.Warn("tableInfo.DestTableInfo or SrcTableInfo is empty", "tableMappingIdx", tiIdx, "SrcTableInfo", tableInfo.SrcTableInfo)
					continue
				}
				destParts := strings.Split(tableInfo.DestTableInfo[0], ".")
				srcParts := strings.Split(tableInfo.SrcTableInfo[0], ".")
				if len(srcParts) > 0 && srcParts[0] == dbConnInfo.Name {
					if len(destParts) > 2 {
						ruleName := "r_" + destParts[2]
						instance.RouteRules = append(instance.RouteRules, ruleName)
						slog.Debug("added route rule for non-regex table", "dbName", dbConnInfo.Name, "ruleName", ruleName, "tableMappingIdx", tiIdx)
					} else {
						slog.Warn("dest table info has insufficient parts (<3)", "tableMappingIdx", tiIdx, "DestTableInfo", tableInfo.DestTableInfo[0], "parts", destParts)
						continue
					}
				}
			} else {
				// Loop through each source table info
				for _, src := range tableInfo.SrcTableInfo {
					srcParts := strings.Split(src, ".")
					if len(srcParts) > 0 && srcParts[0] == dbConnInfo.Name {
						destParts := strings.Split(tableInfo.DestTableInfo[0], ".")
						if len(destParts) > 2 {
							ruleName := "r_" + destParts[2]
							instance.RouteRules = append(instance.RouteRules, ruleName)
							slog.Debug("added route rule for regex table", "dbName", dbConnInfo.Name, "ruleName", ruleName, "src", src)
						}
						break // at least one matched, append and stop checking further
					}
				}
			}

			// Loop the tableInfo.SrcTableInfo and add the db name into allowList if it does not exists.
			// The SrcTableInfo format is instanceName.SchemaName.TableName
			for _, src := range tableInfo.SrcTableInfo {
				parts := strings.Split(src, ".")
				if len(parts) > 1 {
					instanceName := parts[0]
					dbName := parts[1]
					if instanceName != dbConnInfo.Name {
						continue
					}
					found := false
					for _, existing := range allowList {
						if existing == dbName {
							found = true
							break
						}
					}
					if !found {
						allowList = append(allowList, dbName)
						slog.Debug("added db to allowList", "dbName", dbName, "instance", dbConnInfo.Name)
					}
				}
			}
		}

		data.MySQLInstances = append(data.MySQLInstances, instance)
		data.AllowList[dbConnInfo.Name] = allowList
		slog.Info("built MySQL instance", "dbName", dbConnInfo.Name, "sourceID", instance.SourceID, "allowList", allowList, "routeRules", instance.RouteRules)
	}

	// Build routes from tableMapping
	for tiIdx, tableInfo := range *tableMapping {
		var schemaPattern, tablePattern string
		if len(tableInfo.DestTableInfo) == 0 {
			slog.Warn("tableInfo.DestTableInfo is empty, skipping route build", "tableMappingIdx", tiIdx, "SrcTableInfo", tableInfo.SrcTableInfo)
			continue
		}
		destParts := strings.Split(tableInfo.DestTableInfo[0], ".")
		if len(destParts) < 3 {
			slog.Warn("dest table info insufficient parts for route", "tableMappingIdx", tiIdx, "DestTableInfo", tableInfo.DestTableInfo[0], "parts", destParts)
			continue
		}
		routeKey := "r_" + destParts[2]
		if tableInfo.SrcRegex == "" {
			if len(tableInfo.SrcTableInfo) == 0 {
				slog.Warn("tableInfo.SrcTableInfo is empty and no SrcRegex", "tableMappingIdx", tiIdx, "DestTableInfo", tableInfo.DestTableInfo)
				continue
			}
			parts := strings.Split(tableInfo.SrcTableInfo[0], ".")
			if len(parts) > 2 {
				schemaPattern = parts[1]
				tablePattern = parts[2]
			} else {
				slog.Warn("src table info insufficient parts for route", "tableMappingIdx", tiIdx, "SrcTableInfo", tableInfo.SrcTableInfo[0], "parts", parts)
				continue
			}
		} else {
			if len(tableInfo.SrcRegex) == 0 {
				slog.Warn("tableInfo.SrcRegex is invalid", "tableMappingIdx", tiIdx, "SrcRegex", tableInfo.SrcRegex)
				continue
			}
			parts := strings.Split(tableInfo.SrcRegex, ".")
			if len(parts) > 1 {
				schemaPattern = parts[0]
				tablePattern = parts[1]
			} else {
				slog.Warn("SrcRegex invalid for route", "tableMappingIdx", tiIdx, "SrcRegex", tableInfo.SrcRegex)
				continue
			}
		}
		data.Routes[routeKey] = RouteRule{
			SchemaPattern: schemaPattern,
			TablePattern:  tablePattern,
			TargetSchema:  destParts[1],
			TargetTable:   destParts[2],
		}
		slog.Debug("built route", "routeKey", routeKey, "schemaPattern", schemaPattern, "tablePattern", tablePattern, "targetSchema", destParts[1], "targetTable", destParts[2])
	}

	// Parse the template
	tmpl, err := template.New("dm-task").Parse(string(tmplBytes))
	if err != nil {
		slog.Error("failed to parse template", "error", err)
		return fmt.Errorf("failed to parse template: %w", err)
	}

	// Create output file
	outputPath := config.Output
	if !strings.HasSuffix(outputPath, "/") {
		outputPath += "/"
	}
	outFileName := outputPath + "dm-task.yaml"
	outFile, err := os.Create(outFileName)
	if err != nil {
		slog.Error("failed to create output file", "file", outFileName, "error", err)
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	// Execute the template
	if err := tmpl.Execute(outFile, data); err != nil {
		slog.Error("failed to execute template", "file", outFileName, "error", err)
		return fmt.Errorf("failed to execute template: %w", err)
	}

	slog.Info("successfully rendered dm-task.yaml", "file", outFileName)
	return nil
}
