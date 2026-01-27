package main

import (
	"fmt"
	"os"
	"strings"
	"text/template"
)

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
}

func RenderSyncDiffConfig(config *Config, tableMapping *[]TableInfo) error {
	if config == nil {
		return fmt.Errorf("config is nil")
	}

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

	// Map config to DataSources
	for _, ds := range config.SourceDB {

		// Collect route rules for this data source
		var routeRules []string
		for _, tableInfo := range *tableMapping {
			// Extract instance name from SrcTableInfo
			destParts := strings.Split(tableInfo.DestTableInfo[0], ".")
			var ruleName string
			if len(destParts) > 2 {
				ruleName = "r_" + destParts[2]
			}
			for _, src := range tableInfo.SrcTableInfo {
				parts := strings.Split(src, ".")
				if len(parts) > 0 && parts[0] == ds.Name {
					// src format: instance.schemaName.TableName -> rule_TableName
					// ruleName := "r_" + parts[len(parts)-1]
					routeRules = append(routeRules, ruleName)
					break // at least one matched, append and stop checking further
				}
			}
		}
		syncDiffConfig.DataSources[ds.Name] = DataSource{
			Host:     ds.Host,
			Port:     ds.Port,
			User:     ds.User,
			Password: ds.Password,
			// TimeZone:   ds.TimeZone,
			// Location:   ds.Location,
			RouteRules: routeRules,
		}
	}

	// Map config.DestDB (single struct) to DataSources as well
	syncDiffConfig.DataSources[config.DestDB.Name] = DataSource{
		Host:     config.DestDB.Host,
		Port:     config.DestDB.Port,
		User:     config.DestDB.User,
		Password: config.DestDB.Password,
		// TimeZone:   config.DestDB.TimeZone,
		// Location:   config.DestDB.Location,
		// RouteRules: config.DestDB.RouteRules,
	}

	// Map tableMapping to syncDiffConfig's Routes. If SrcRegex is not empty, use it as the TablePattern.
	for _, tableInfo := range *tableMapping {
		// routeKey := fmt.Sprintf("%s:%s", tableInfo.SrcTableInfo[0], tableInfo.DestTableInfo[0])
		var schemaPattern, tablePattern string
		routeKey := "r_" + strings.Split(tableInfo.DestTableInfo[0], ".")[2]
		if tableInfo.SrcRegex == "" {
			parts := strings.Split(tableInfo.SrcTableInfo[0], ".")
			if len(parts) > 2 {
				schemaPattern = parts[1]
				tablePattern = parts[2]
			}
		} else {
			parts := strings.Split(tableInfo.SrcRegex, ".")
			if len(parts) > 1 {
				schemaPattern = parts[0]
				tablePattern = parts[1]
			}
		}
		syncDiffConfig.Routes[routeKey] = RouteRule{
			SchemaPattern: schemaPattern,
			TablePattern:  tablePattern,
			TargetSchema:  strings.Split(tableInfo.DestTableInfo[0], ".")[1],
			TargetTable:   strings.Split(tableInfo.DestTableInfo[0], ".")[2],
		}
	}

	// Build source-instances list from config.SourceDB names
	sourceInstances := make([]string, 0, len(config.SourceDB))
	for _, ds := range config.SourceDB {
		sourceInstances = append(sourceInstances, ds.Name)
	}

	// Build target-check-tables list from tableMapping DestTableInfo
	targetCheckTables := make([]string, 0, len(*tableMapping))
	for _, tableInfo := range *tableMapping {
		if len(tableInfo.DestTableInfo) > 0 {
			// Convert instance.schemaName.TableName to schemaName.TableName
			parts := strings.SplitN(tableInfo.DestTableInfo[0], ".", 3)
			if len(parts) == 3 {
				targetCheckTables = append(targetCheckTables, parts[1]+"."+parts[2])
			} else {
				targetCheckTables = append(targetCheckTables, tableInfo.DestTableInfo[0])
			}
		}
	}
	// Set Task field
	syncDiffConfig.Task = TaskConfig{
		// OutputDir:         config.OutputDir,
		OutputDir:         "./output",
		SourceInstances:   sourceInstances,
		TargetInstance:    config.DestDB.Name,
		TargetCheckTables: targetCheckTables,
	}

	var mapExcludeColumns = make(map[string][]string)
	// Build mapExcludeColumns: key is c_instance / c_instance,c_schema / c_instance,c_schema,c_table
	// depending on tableMapping element's DestHasSource / DestHasSchema / DestHasTableName flags.
	// The value is the list of columns to exclude from the check.
	for _, tbl := range *tableMapping {
		var key string
		destParts := strings.Split(tbl.DestTableInfo[0], ".")
		if len(destParts) < 3 {
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
			continue
		}
		key = strings.Join(parts, ",")
		mapExcludeColumns[key] = append(mapExcludeColumns[key], schema+"."+table)
	}

	// Loop mapExcludeColumns and prepare the TableConfigs
	idx := 0
	for key, tables := range mapExcludeColumns {
		// Determine which columns to ignore based on the key
		var ignoreCols = strings.Split(key, ",")

		// Create a TableConfig entry for this key
		cfgKey := fmt.Sprintf("cfg_%d", idx)
		syncDiffConfig.TableConfigs[cfgKey] = TableConfig{
			TargetTables:  tables,
			IgnoreColumns: ignoreCols,
		}

		syncDiffConfig.Task.TargetConfigs = append(syncDiffConfig.Task.TargetConfigs, cfgKey)
		idx = idx + 1
	}

	fmt.Printf("%+v\n", syncDiffConfig)

	// Read the template file
	// Create or open the output file
	outFile, err := os.Create("test.toml")
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	// Read the template file
	tmplBytes, err := os.ReadFile("templates/diff.tpl.toml")
	if err != nil {
		return fmt.Errorf("failed to read template file: %w", err)
	}

	// Parse the template
	tmpl, err := template.New("diff").Parse(string(tmplBytes))
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	// Execute the template with the config data into the file
	if err := tmpl.Execute(outFile, syncDiffConfig); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}
	return nil
}

func RenderDMSourceConfig(config *Config) error {
	if config == nil {
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

	// Loop over each source DB and generate a config file
	for i, db := range config.SourceDB {
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
			return fmt.Errorf("failed to parse DM template: %w", err)
		}

		// Create output file
		outFileName := fmt.Sprintf("dm-source-%s.yaml", db.Name)
		outFile, err := os.Create(outFileName)
		if err != nil {
			return fmt.Errorf("failed to create output file %s: %w", outFileName, err)
		}
		defer outFile.Close()

		if err := tmpl.Execute(outFile, data); err != nil {
			return fmt.Errorf("failed to execute template for %s: %w", db.Name, err)
		}
	}

	return nil
}

func RenderDMTaskConfig(config *Config, tableMapping *[]TableInfo) error {
	if config == nil {
		return fmt.Errorf("config is nil")
	}

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
			SourceID   string
			RouteRules []string
		}
		Validators struct {
			Mode        string
			WorkerCount int
			ErrorDelay  string
		}
	}

	// Read the template file
	tmplBytes, err := os.ReadFile("templates/task.tpl.toml")
	if err != nil {
		return fmt.Errorf("failed to read template file: %w", err)
	}

	// Prepare template data
	data := DMTaskTemplateData{
		Name:       "dm-task",
		TaskMode:   "all",
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
	}

	// Build MySQL instances
	for i, dbConnInfo := range config.SourceDB {
		instance := struct {
			SourceID   string
			RouteRules []string
		}{
			SourceID: fmt.Sprintf("mysql-sourcedb-%d", 10000+i),
		}

		// Collect route rules for this instance
		for _, tableInfo := range *tableMapping {
			if tableInfo.SrcRegex == "" {
				destParts := strings.Split(tableInfo.DestTableInfo[0], ".")
				srcParts := strings.Split(tableInfo.SrcTableInfo[0], ".")
				if len(srcParts) > 0 && srcParts[0] == dbConnInfo.Name {
					if len(destParts) > 2 {
						ruleName := "r_" + destParts[2]
						instance.RouteRules = append(instance.RouteRules, ruleName)
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
						}
						break // at least one matched, append and stop checking further
					}
				}
			}

		}

		data.MySQLInstances = append(data.MySQLInstances, instance)
	}

	// Parse the template
	tmpl, err := template.New("dm-task").Parse(string(tmplBytes))
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	// Create output file
	outFile, err := os.Create("dm-task.yaml")
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	// Execute the template
	if err := tmpl.Execute(outFile, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}
