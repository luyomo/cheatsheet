package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	openai "github.com/sashabaranov/go-openai"
	"github.com/spf13/cobra"
	"log"
	"os"
	"regexp"
	"strings"
	"text/template"

	_ "github.com/go-sql-driver/mysql"
)

type DBConnInfo struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
}

type TableInfo struct {
	MD5Columns          string
	MD5ColumnsWithTypes string
	SrcRegex            string
	SrcTableInfo        []string
	DestTableInfo       []string
}

var (
	strTpl     string
	srcDBInfo  DBConnInfo
	destDBInfo DBConnInfo
	outputFile string
	outputErr  string
)

var rootCmd = &cobra.Command{
	Use:   "md-toolkit",
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

	// Define flags for source and destination databases
	rootCmd.PersistentFlags().StringVar(&srcDBInfo.Host, "src-host", "", "Source database host")
	rootCmd.PersistentFlags().IntVar(&srcDBInfo.Port, "src-port", 4000, "Source database port")
	rootCmd.PersistentFlags().StringVar(&srcDBInfo.User, "src-user", "", "Source database user")
	rootCmd.PersistentFlags().StringVar(&srcDBInfo.Password, "src-password", "", "Source database password")
	rootCmd.PersistentFlags().StringVar(&srcDBInfo.DBName, "src-dbs", "", "Source database name")

	rootCmd.PersistentFlags().StringVar(&destDBInfo.Host, "dest-host", "", "Destination database host")
	rootCmd.PersistentFlags().IntVar(&destDBInfo.Port, "dest-port", 4000, "Destination database port")
	rootCmd.PersistentFlags().StringVar(&destDBInfo.User, "dest-user", "", "Destination database user")
	rootCmd.PersistentFlags().StringVar(&destDBInfo.Password, "dest-password", "", "Destination database password")
	rootCmd.PersistentFlags().StringVar(&destDBInfo.DBName, "dest-dbs", "", "Destination database name")

	rootCmd.PersistentFlags().StringVar(&outputFile, "output", "", "Output file path")
	rootCmd.PersistentFlags().StringVar(&outputFile, "error-file", "", "Output file path for failed mapping tables")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}

	if strTpl == "" {
		fmt.Printf("Please provide template command for dumpling \n")
		return
	}

	tableStructure := []TableInfo{}

	// fmt.Printf("template: %s \n", strTpl)
	// return

	err := fetch_table_def("source", &tableStructure, srcDBInfo, []string{
		"orderdb_01",
	})
	if err != nil {
		fmt.Printf("Failed to fetch table definition: %v \n", err)
		return
	}

	// fmt.Printf("Source table info: %#v \n\n", tableStructure)
	err = fetch_table_def("dest", &tableStructure, destDBInfo, []string{
		"tidb_orderdb",
	})

	// fmt.Printf("Dest table info: %#v \n", tableStructure)

	for _, tableInfo := range tableStructure {
		fmt.Printf("md5: %s, md5 with type: %s, source table: %#v, dest tables: %#v \n", tableInfo.MD5Columns, tableInfo.MD5ColumnsWithTypes, tableInfo.SrcTableInfo, tableInfo.DestTableInfo)
	}

	tmpl := template.Must(template.New("dumpling").Parse(strTpl))

	// Open output file for writing if specified
	var outputWriter *os.File
	if outputFile != "" {
		var err error
		outputWriter, err = os.Create(outputFile)
		if err != nil {
			log.Fatalf("Failed to create output file: %v", err)
		}
		defer outputWriter.Close()
	} else {
		outputWriter = os.Stdout
	}

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

	// fmt.Printf("--------- Start to prepare dumpling command ----- ---- \n")
	for _, tableInfo := range tableStructure {
		// Case 1: One-to-one mapping
		if len(tableInfo.SrcTableInfo) == 1 && len(tableInfo.DestTableInfo) == 1 {
			srcTable := tableInfo.SrcTableInfo[0]
			destTable := tableInfo.DestTableInfo[0]
			// fmt.Printf("dumpling -h 10.0.3.7 -P 4000 -u root -p '1234Abcd' --threads 8  --tables-list '%s' --output-filename-template '%s.{{.Index}}' --filetype csv -o 'azblob://tidbdataimport/merged_table_test/' --azblob.account-name jaytest001 --azblob.sas-token '${SAS}'\n", srcTable, destTable)
			data := struct {
				SrcTable  string
				DestTable string
			}{
				SrcTable:  srcTable,
				DestTable: fmt.Sprintf("%s.{{.Index}}", destTable),
			}

			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, data); err != nil {
				log.Printf("Error executing template: %v", err)
			}
			fmt.Fprintf(outputWriter, "%s\n", buf.String())
			// TODO: Implement dumpling command generation
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

				// Find matching destination table
				for j := 0; j < len(tableInfo.DestTableInfo); j++ {
					destParts := strings.Split(tableInfo.DestTableInfo[j], ".")
					destTableName := destParts[len(destParts)-1]

					if srcTableName == destTableName {
						// fmt.Printf("Map %s -> %s\n", tableInfo.SrcTableInfo[i], tableInfo.DestTableInfo[j])

						// fmt.Printf("dumpling -h 10.0.3.7 -P 4000 -u root -p '1234Abcd' --threads 8  --tables-list '%s' --output-filename-template '%s.{{.Index}}' --filetype csv -o 'azblob://tidbdataimport/merged_table_test/' --azblob.account-name jaytest001 --azblob.sas-token '${SAS}'\n", tableInfo.SrcTableInfo[i], tableInfo.DestTableInfo[j])
						data := struct {
							SrcTable  string
							DestTable string
						}{
							SrcTable:  tableInfo.SrcTableInfo[i],
							DestTable: fmt.Sprintf("%s.{{.Index}}", tableInfo.DestTableInfo[j]),
						}

						var buf bytes.Buffer
						if err := tmpl.Execute(&buf, data); err != nil {
							log.Printf("Error executing template: %v", err)
						}
						// fmt.Printf("%s\n", buf.String())
						fmt.Fprintf(outputWriter, "%s\n", buf.String())
						break
					}
				}
			}
			// TODO: Implement table mapping logic
		}

		// Case 3: Many-to-one consolidation
		if len(tableInfo.SrcTableInfo) > 1 && len(tableInfo.DestTableInfo) == 1 {
			destTable := tableInfo.DestTableInfo[0]
			// fmt.Printf("Using consolidation process to merge tables into %s:\n", destTable)
			for idx, srcTable := range tableInfo.SrcTableInfo {

				data := struct {
					SrcTable  string
					DestTable string
				}{
					SrcTable:  srcTable,
					DestTable: fmt.Sprintf("%s.%04d{{.Index}}", destTable, idx+1),
				}

				var buf bytes.Buffer
				if err := tmpl.Execute(&buf, data); err != nil {
					log.Printf("Error executing template: %v", err)
				}
				// fmt.Printf("%s\n", buf.String())
				fmt.Fprintf(outputWriter, "%s\n", buf.String())
				// fmt.Printf("dumpling -h 10.0.3.7 -P 4000 -u root -p '1234Abcd' --threads 8  --tables-list '%s' --output-filename-template '%s.%04d{{.Index}}' --filetype csv -o 'azblob://tidbdataimport/merged_table_test/' --azblob.account-name jaytest001 --azblob.sas-token '${SAS}'\n", srcTable, destTable, idx+1)
			}
			// TODO: Implement consolidation logic
		}
	}

	return

	// // The Data Source Name (DSN) string
	// // Format: "user:password@tcp(host:port)/database?param=value"
	// // Replace with your actual database credentials
	// dsn := "root:1234Abcd@tcp(10.0.3.7:4000)/orderdb_01"

	// // 1. Open a database handle
	// // This does not yet establish a connection, but it prepares the database object.
	// db, err := sql.Open("mysql", dsn)
	// if err != nil {
	// 	log.Fatalf("Failed to open database connection: %v", err)
	// }
	// // Ensure the connection is closed when the main function exits.
	// defer db.Close()

	// // 2. Ping the database to verify the connection
	// // This performs a real check to see if the database is reachable.
	// if err := db.Ping(); err != nil {
	// 	log.Fatalf("Failed to ping database: %v", err)
	// }

	// fmt.Println("Successfully connected to the MySQL database!")

	// // 2. Define the SQL query with placeholders
	// query := `
	// 	SELECT
	// 	    TABLE_SCHEMA,
	// 		TABLE_NAME,
	// 		MD5(GROUP_CONCAT(COLUMN_NAME ORDER BY COLUMN_NAME ASC SEPARATOR ',')),
	// 		MD5(GROUP_CONCAT(CONCAT_WS(':',
	// 		    COLUMN_NAME,
	//             COLUMN_TYPE,
	//             COLUMN_DEFAULT,
	//             IS_NULLABLE,
	//             CHARACTER_MAXIMUM_LENGTH,
	//             NUMERIC_PRECISION,
	//             NUMERIC_SCALE,
	//             DATETIME_PRECISION) ORDER BY COLUMN_NAME ASC SEPARATOR ','))
	// 	 FROM INFORMATION_SCHEMA.COLUMNS
	// 	WHERE TABLE_SCHEMA = ?
	// 	GROUP BY TABLE_SCHEMA, TABLE_NAME;
	// `

	// // 3. Define the database and table you want to query
	// databaseName := "orderdb_01"
	// // tableName := "your_table_name"

	// // 4. Prepare the SQL statement to prevent SQL injection
	// stmt, err := db.Prepare(query)
	// if err != nil {
	// 	log.Fatalf("Failed to prepare statement: %v", err)
	// }
	// defer stmt.Close()

	// // 5. Execute the query with the table names as parameters
	// rows, err := stmt.Query(databaseName)
	// if err != nil {
	// 	log.Fatalf("Failed to execute query: %v", err)
	// }
	// defer rows.Close()

	// type TableInfo struct {
	// 	MD5Columns          string
	// 	MD5ColumnsWithTypes string
	// 	Regex               string
	// 	TableInfo           []string
	// }

	// // tableStructure := []TableInfo{}
	// // "md5Columns":         md5Columns,
	// // "md5ColumnsWithTypes": md5ColumnsWithTypes,
	// // "tableInfo":          []string{fmt.Sprintf("%s.%s", tableSchema, tableName)},
	// // }

	// // 6. Iterate through the results
	// for rows.Next() {
	// 	var tableSchema, tableName, md5Columns, md5ColumnsWithTypes string
	// 	if err := rows.Scan(&tableSchema, &tableName, &md5Columns, &md5ColumnsWithTypes); err != nil {
	// 		log.Fatalf("Failed to scan row: %v", err)
	// 	}
	// 	// fmt.Printf("Schema: %s, TableName: %s, MD5 of columns: %s, types: %s \n",
	// 	// 	tableSchema, tableName, md5Columns, md5ColumnsWithTypes)

	// 	// Create new TableInfo struct and append to slice
	// 	newTableInfo := TableInfo{
	// 		MD5Columns:          md5Columns,
	// 		MD5ColumnsWithTypes: md5ColumnsWithTypes,
	// 		// RegrEx:             fmt.Sprintf("%s:%s", md5Columns, md5ColumnsWithTypes),
	// 		TableInfo: []string{fmt.Sprintf("%s.%s", tableSchema, tableName)},
	// 	}

	// 	// Check if similar table structure exists
	// 	found := false
	// 	for i, existing := range tableStructure {
	// 		if existing.MD5Columns == newTableInfo.MD5Columns &&
	// 			existing.MD5ColumnsWithTypes == newTableInfo.MD5ColumnsWithTypes {
	// 			*tableStructure[i].TableInfo = append(tableStructure[i].TableInfo,
	// 				fmt.Sprintf("%s.%s", tableSchema, tableName))
	// 			found = true
	// 			break
	// 		}
	// 	}

	// 	// If no match found, append new structure
	// 	if !found {
	// 		tableStructure = append(tableStructure, newTableInfo)
	// 	}
	// }

	// if err := rows.Err(); err != nil {
	// 	log.Fatalf("Error occurred during row iteration: %v", err)
	// }

	// for idx := range tableStructure {
	// 	if len(tableStructure[idx].TableInfo) > 5 {
	// 		regex, err := generateRegex(tableStructure[idx].TableInfo)
	// 		if err != nil {
	// 			fmt.Printf("------ Error generating regex: %v\n", err)
	// 		}
	// 		if regex != nil {
	// 			tableStructure[idx].Regex = *regex
	// 		} else {
	// 			fmt.Printf("Failed to detect the regex")
	// 		}
	// 	}
	// }

	// // Print the table structure
	// for _, tableInfo := range tableStructure {
	// 	fmt.Printf("md5: %s, md5 with type: %s, regex: %s, num of tables: %d, tables: %s \n", tableInfo.MD5Columns, tableInfo.MD5ColumnsWithTypes, tableInfo.Regex, len(tableInfo.TableInfo), strings.Join(tableInfo.TableInfo, ", "))
	// }
}

type RegexResult struct {
	Regex string `json:"regex"`
}

type ChatHistory struct {
	Messages []openai.ChatCompletionMessage
}

func (ch *ChatHistory) AddMessage(role string, content string) {
	ch.Messages = append(ch.Messages, openai.ChatCompletionMessage{
		Role:    role,
		Content: content,
	})
}

func (ch *ChatHistory) AddResponse(msg openai.ChatCompletionMessage) {
	ch.Messages = append(ch.Messages, msg)
}

func generateRegex(tables []string) (*string, error) {
	client := openai.NewClient(os.Getenv("OPENAI_API_KEY"))

	// fmt.Printf("What is the regex for the following tables: %v \n", strings.Join(tables, ",") )

	// 1. Define the tool with the function schema
	tools := []openai.Tool{
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "regex_is_valid",
				Description: "Verify if the given regex matches all of the provided tables. Return unmatched tables if any or error message.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"regex": map[string]interface{}{
							"type":        "string",
							"description": "The candidate regex to validate. Should be anchored with ^ and $.",
						},
					},
					"required": []string{"regex"},
				},
			},
		},
	}

	// 2) Conversation seed: instruct the model to iterate until valid
	system := openai.ChatCompletionMessage{
		Role: openai.ChatMessageRoleSystem,
		Content: strings.Join([]string{
			"You generate a single regex that matches ALL and ONLY the user's tables.",
			"Rules:",
			"- Always use ^ and $ anchors to match the entire string.",
			"- Prefer a concise pattern.",
			"- Use ToolCalls `regex_is_valid` with your candidate regex (and pass the tables).",
			"- If the ToolCalls(regex_is_valid) returns unmatched tables or error, refine your regex and call the tool again.",
			"- Repeat until it validates or you cannot improve. ",
			"- Only return the result without any comment",
		}, "\n"),
	}

	user := openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: fmt.Sprintf("Tables:\n%s", strings.Join(tables, "\n")),
	}

	// expectedRegex := ""

	messages := []openai.ChatCompletionMessage{system, user}
	const maxRounds = 6
	for round := 1; round <= maxRounds; round++ {
		resp, err := client.CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{
			Model:    openai.GPT3Dot5Turbo, // or your preferred model
			Messages: messages,
			Tools:    tools,
		})
		if err != nil {
			log.Fatalf("ChatCompletion error (round %d): %v", round, err)
		}

		assistant := resp.Choices[0].Message

		// fmt.Printf("Round %d: %#v \n", round, assistant)
		// fmt.Printf("    toolCalles: %#v \n", assistant.ToolCalls)

		// If the assistant wants to call tools, run them and feed results back
		if len(assistant.ToolCalls) > 0 {
			// Add the assistant message with tool_calls to history
			messages = append(messages, assistant)

			for _, tc := range assistant.ToolCalls {
				if tc.Function.Name != "regex_is_valid" {
					continue
				}

				// Parse arguments
				var args RegexResult
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
					// If parsing fails, give the model a helpful error signal
					toolContent := ToolReturn{Valid: false, Error: "Bad JSON arguments for regex_is_valid"}
					contentBytes, _ := json.Marshal(toolContent)
					messages = append(messages, openai.ChatCompletionMessage{
						Role:       openai.ChatMessageRoleTool,
						ToolCallID: tc.ID,
						Content:    string(contentBytes),
					})
					continue
				}

				// Run your local validator
				toolContent := regex_is_valid(args.Regex, tables)
				// fmt.Printf("Checking the regex_is_valid tool done %#v \n", toolContent)
				if toolContent.Valid {
					// fmt.Printf("Final regex: %s \n", args.Regex)
					return &toolContent.Regex, nil
				}
				// fmt.Printf("Tables checked: %v \n", toolContent)

				// Return the result to the model
				// toolContent := ToolReturn{Valid: valid, Unmatched: unmatched}
				contentBytes, _ := json.Marshal(toolContent)

				messages = append(messages, openai.ChatCompletionMessage{
					Role:       openai.ChatMessageRoleTool,
					ToolCallID: tc.ID,
					Content:    string(contentBytes),
				})
			}

		}
		// else {
		// 	// // No tool_calls: this should be the model's final answer (the regex or an explanation)
		// // fmt.Println("Final assistant message:")
		// // fmt.Printf("All the result: %#v \n", resp.Choices[0])
		// // fmt.Printf("------------------- \n")
		// // fmt.Println(assistant.Content)
		// // fmt.Printf("=================== \n")
		// return &expectedRegex, nil
		// }

	}

	fmt.Println("Stopped after max rounds without a final answer.")

	return nil, nil
}

type ToolReturn struct {
	Regex     string   `json:"regex"`
	Valid     bool     `json:"valid"`
	Unmatched []string `json:"unmatched"`
	Error     string   `json:"error"`
}

func regex_is_valid(regex string, tables []string) ToolReturn {
	result := ToolReturn{
		Regex:     regex,
		Valid:     true,
		Unmatched: []string{},
		Error:     "",
	}

	re, err := regexp.Compile(regex)
	if err != nil {
		result.Valid = false
		result.Error = fmt.Sprintf("invalid regex: %v", err)
		return result
	}

	for _, t := range tables {
		if !re.MatchString(t) {
			result.Valid = false
			result.Unmatched = append(result.Unmatched, t)
		}
	}

	return result
}

func fetch_table_def(tableType string, tableStructure *[]TableInfo, dbInfo DBConnInfo, targetDBs []string) error {
	// The Data Source Name (DSN) string
	// Format: "user:password@tcp(host:port)/database?param=value"
	// Replace with your actual database credentials
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", dbInfo.User, dbInfo.Password, dbInfo.Host, dbInfo.Port, dbInfo.DBName)

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

	fmt.Println("Successfully connected to the MySQL database!")

	// 2. Define the SQL query with placeholders
	// case when upper(COLUMN_TYPE) IN ('BIGINT', 'INT', 'MEDIUMINT', 'SMALLINT', 'TINYINT') then '0' else NUMERIC_PRECISION end
	// create table (..., col1 int(2) ...) -> the ddl is converted to create table (..., col1 int ...). Compatible to MySQL 8.0
	query := fmt.Sprintf(`
		SELECT
		    TABLE_SCHEMA, 
			TABLE_NAME,
			MD5(GROUP_CONCAT(COLUMN_NAME ORDER BY COLUMN_NAME ASC SEPARATOR ',')),
			MD5(GROUP_CONCAT(CONCAT_WS(':',         
			    COLUMN_NAME,
                COLUMN_TYPE,
                COLUMN_DEFAULT,
                IS_NULLABLE,
                CHARACTER_MAXIMUM_LENGTH,
                case when upper(COLUMN_TYPE) IN ('BIGINT', 'INT', 'MEDIUMINT', 'SMALLINT', 'TINYINT') then '0' else NUMERIC_PRECISION end,
                NUMERIC_SCALE,
                DATETIME_PRECISION) ORDER BY COLUMN_NAME ASC SEPARATOR ','))
		 FROM INFORMATION_SCHEMA.COLUMNS
		WHERE TABLE_SCHEMA = '%s' 
		GROUP BY TABLE_SCHEMA, TABLE_NAME
	`, strings.Join(targetDBs, "','"))

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
		if err := rows.Scan(&tableSchema, &tableName, &md5Columns, &md5ColumnsWithTypes); err != nil {
			log.Fatalf("Failed to scan row: %v", err)
		}

		// Create new TableInfo struct and append to slice
		newTableInfo := TableInfo{
			MD5Columns:          md5Columns,
			MD5ColumnsWithTypes: md5ColumnsWithTypes,
		}
		if tableType == "source" {
			newTableInfo.SrcTableInfo = []string{fmt.Sprintf("%s.%s", tableSchema, tableName)}
		} else {
			newTableInfo.DestTableInfo = []string{fmt.Sprintf("%s.%s", tableSchema, tableName)}
		}

		// Check if similar table structure exists
		found := false
		for i, existing := range *tableStructure {
			if existing.MD5Columns == newTableInfo.MD5Columns &&
				existing.MD5ColumnsWithTypes == newTableInfo.MD5ColumnsWithTypes {
				if tableType == "source" {
					(*tableStructure)[i].SrcTableInfo = append((*tableStructure)[i].SrcTableInfo,
						fmt.Sprintf("%s.%s", tableSchema, tableName))
				} else {
					(*tableStructure)[i].DestTableInfo = append((*tableStructure)[i].DestTableInfo,
						fmt.Sprintf("%s.%s", tableSchema, tableName))
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

/*
 * TODO:
 *   1. Prepare the destination tables in the destination database.
 *   2. Calculate the same struct as the source in the destination database.
 *   3. Map the source table to the destination table using the md5 and table names.
 *   3.1 If the md5 relationship is m-m, use the table name to match. Fail to match through it out.
 *   3.2 If the relationship is is n-1, use the consolidation.
 *   3.3 If the relationship is 1-n, throw it out.
 *   4. Consolidate all the 1-1 relationships table into one group.
 *   5. Use the mergin logic to generate the mapping.
 *   target: ./dumpling statement
 *            dm's mapping rule
 */
