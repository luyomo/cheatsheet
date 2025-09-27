package main

import (
    "encoding/csv"
    "fmt"
    "io"
    "os"
    "path/filepath"
    "strconv"
    "strings"
)

func main() {
    if len(os.Args) != 4 {
        fmt.Println("Usage: go run main.go <schema_name> <table_name> <folder_path>")
        return
    }
    schemaName := os.Args[1]
    tableName := os.Args[2]
    folderPath := os.Args[3]

    // Make IDs unique in all CSV files
    if err := makeIDsUnique(schemaName, tableName, folderPath); err!= nil {
        fmt.Println("Error:", err)
        return
    }
    fmt.Println("IDs have been made unique in all CSV files.")
}

func makeIDsUnique(schemaName, tableName, folderPath string) error {
    // Get all CSV files matching the pattern
    pattern := fmt.Sprintf("%s.%s.*.csv", schemaName, tableName)
    files, err := filepath.Glob(filepath.Join(folderPath, pattern))
    if err != nil {
        return fmt.Errorf("error finding CSV files: %v", err)
    }

	// Read checkpoint file
    checkpointFile := filepath.Join(folderPath, ".checkpoint")
    processedFiles := make(map[string]int64)
    var maxProcessedID int64 = 0
	if data, err := os.ReadFile(checkpointFile); err == nil {
        lines := strings.Split(string(data), "\n")
        for _, line := range lines {
            if line == "" {
                continue
            }
            parts := strings.Split(line, ",")
            if len(parts) == 2 {
                if id, err := strconv.ParseInt(parts[1], 10, 64); err == nil {
                    processedFiles[parts[0]] = id
                    if id > maxProcessedID {
                        maxProcessedID = id
                    }
                }
            }
        }
    }

    idCounter := maxProcessedID + 1
    
    // Process each file
    for _, file := range files {
		if _, exists := processedFiles[file]; exists {
            continue
        }

        // Open original file
        f, err := os.Open(file)
        if err != nil {
            return fmt.Errorf("error opening file %s: %v", file, err)
        }
        
        // Create temporary output file
        tempFile := file + ".tmp"
        out, err := os.Create(tempFile)
        if err != nil {
            f.Close()
            return fmt.Errorf("error creating temp file: %v", err)
        }

        reader := csv.NewReader(f)
        writer := csv.NewWriter(out)
	
        // Read and process each row
        firstRow := true
        maxID := idCounter
        for {
            record, err := reader.Read()
            if err == io.EOF {
                break
            }
            if err != nil {
                f.Close()
                out.Close()
                os.Remove(tempFile)
                return fmt.Errorf("error reading CSV: %v", err)
            }

            if firstRow {
                // Write header row as-is
                firstRow = false
            } else {
                // Replace ID with new unique ID
                record[0] = strconv.FormatInt(idCounter, 10)
                maxID = idCounter
                idCounter++
            }

            if err := writer.Write(record); err != nil {
                f.Close()
                out.Close()
                os.Remove(tempFile)
                return fmt.Errorf("error writing CSV: %v", err)
            }
        }

        writer.Flush()
        if err := writer.Error(); err != nil {
            f.Close()
            out.Close()
            os.Remove(tempFile)
            return fmt.Errorf("error flushing writer: %v", err)
        }

        f.Close()
        out.Close()

        // Replace original file with temporary file
        if err := os.Rename(tempFile, file); err != nil {
            os.Remove(tempFile)
            return fmt.Errorf("error replacing original file: %v", err)
        }

        // Save max ID to checkpoint file
        checkpointFile := filepath.Join(filepath.Dir(file), ".checkpoint")
        checkpointContent := fmt.Sprintf("%s,%d", file, maxID)
        
        // Read existing content if file exists
        existingContent := ""
        if data, err := os.ReadFile(checkpointFile); err == nil {
            existingContent = string(data)
        }
        
        // Append new content to existing content
        if existingContent != "" {
            checkpointContent = existingContent + "\n" + checkpointContent
        }
        
        if err := os.WriteFile(checkpointFile, []byte(checkpointContent), 0644); err != nil {
            return fmt.Errorf("error writing checkpoint file: %v", err)
        }
    }

    return nil
}
