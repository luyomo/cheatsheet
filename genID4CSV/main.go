package main

import (
	"encoding/csv"
	"fmt"
	"github.com/spf13/cobra"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"context"
	// "sync"
	// "github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
	// "github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/sas"
	// "net/url"
	"regexp"
	"time"
	// "github.com/dustin/go-humanize"
	"github.com/jedib0t/go-pretty/v6/table"
	// "github.com/gosuri/uiprogress"
)

var (
	storageType   string
	folderPath    string
	storageName   string
	containerName string
	sasToken      string
	schemaName    string
	tableName     string
	rootCmd       *cobra.Command
)

func init() {
	rootCmd = &cobra.Command{
		Use:   "gen-unique-id",
		Short: "Generate unique IDs for CSV files across different storage types",
		Run: func(cmd *cobra.Command, args []string) {
			// Validate required flags
			if storageType == "" {
				cmd.PrintErr("storage type is required")
				return
			}
			if schemaName == "" || tableName == "" {
				cmd.PrintErr("schema name and table name are required")
				return
			}

			// Validate storage-specific requirements
			switch storageType {
			case "local":
				if folderPath == "" {
					cmd.PrintErr("folder path is required for local storage")
					return
				}
				// Call the main processing function
				processState, err := makeIDsUnique4Local(schemaName, tableName, folderPath)
				printSummary(processState)
				if err != nil {
					cmd.PrintErrf("Error: %v\n", err)
					return
				}
			case "azure":
				if storageName == "" || containerName == "" || sasToken == "" || folderPath == "" {
					cmd.PrintErr("storage name, container name, folderPath and SAS token are required for Azure storage")
					return
				}

				processState, err := makeIDsUnique4AZ(storageName, containerName, folderPath, sasToken, schemaName, tableName, ".tmpdata")

				printSummary(processState)
				if err != nil {
					cmd.PrintErrf("Error: %v\n", err)
					return
				}
			case "s3", "gcs":
				// Add specific validation for s3 and gcs if needed
			default:
				cmd.PrintErr("invalid storage type. Must be one of: local, azure, s3, gcs")
				return
			}
		},
	}

	// Define flags
	rootCmd.Flags().StringVarP(&storageType, "storage", "s", "", "Storage type (local, azure, s3, gcs)")
	rootCmd.Flags().StringVarP(&folderPath, "path", "p", "", "Local folder path (required for local storage)")
	rootCmd.Flags().StringVarP(&storageName, "storage-name", "", "", "Storage Name")
	rootCmd.Flags().StringVarP(&containerName, "container", "c", "", "Azure container name")
	rootCmd.Flags().StringVar(&sasToken, "sas-token", "", "Azure SAS token")
	rootCmd.Flags().StringVar(&schemaName, "schema", "", "Schema name")
	rootCmd.Flags().StringVar(&tableName, "table", "", "Table name")

	// Mark required flags
	rootCmd.MarkFlagRequired("storage")
	rootCmd.MarkFlagRequired("schema")
	rootCmd.MarkFlagRequired("table")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func makeIDsUnique4AZ(storageName, containerName, remotePath, sas, schemaName, tableName, localPath string) ([]map[string]string, error) {
	files, err := ListFilteredBlobs(storageName, containerName, sasToken, remotePath, fmt.Sprintf("%s.%s.*.csv", schemaName, tableName))
	if err != nil {
		return nil, err
	}

	// Create local directory if it doesn't exist
	if err := os.MkdirAll(localPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create local directory %s: %w", localPath, err)
	}

	mapState := []map[string]string{}

	var maxID int64 = 1
	for _, file := range files {
		theMapState := map[string]string{
			"file_name":      file,
			"file_size":      "",
			"state":          "pending",
			"execution_time": "",
		}

		startTime := time.Now()

		// fmt.Printf("Processing file: %s\n", file)
		// Download the file to local path
		localFilePath := filepath.Join(localPath, filepath.Base(file))
		if err := DownloadBlob(storageName, sasToken, containerName, file, localFilePath); err != nil {
			executionTime := time.Since(startTime)
			theMapState["execution_time"] = executionTime.String()
			theMapState["state"] = "download failed"
			mapState = append(mapState, theMapState)
			return mapState, fmt.Errorf("failed to download file %s: %w", file, err)
		}
		// Get file size
		fileInfo, err := os.Stat(localFilePath)
		theMapState["file_size"] = formatSize(fileInfo.Size())

		maxID, err = makeUniqeID4File(localFilePath, maxID)
		if err != nil {
			executionTime := time.Since(startTime)
			theMapState["execution_time"] = executionTime.String()
			theMapState["state"] = "processing failed"
			mapState = append(mapState, theMapState)
			return mapState, fmt.Errorf("failed to process file %s: %w", file, err)
		}

		// Upload the processed file back to Azure
		if err := UploadBlobWithSAS(storageName, containerName, file, sasToken, localFilePath); err != nil {
			executionTime := time.Since(startTime)
			theMapState["execution_time"] = executionTime.String()
			theMapState["state"] = "upload failed"
			mapState = append(mapState, theMapState)

			return mapState, fmt.Errorf("failed to upload file %s: %w", file, err)
		}

		executionTime := time.Since(startTime)
		theMapState["execution_time"] = executionTime.String()
		theMapState["state"] = "completed"
		mapState = append(mapState, theMapState)
	}

	// Remove the temporary local directory if processing completed successfully
	if err := os.RemoveAll(localPath); err != nil {
		return nil, fmt.Errorf("failed to remove temporary directory %s: %w", localPath, err)
	}

	return mapState, nil
}

func ListFilteredBlobs(
	// ctx context.Context,
	accountName string,
	containerName string,
	sasToken string,
	pathPrefix string,
	pattern string,
) ([]string, error) {

	// --- 1. Client Setup (using SAS URL) ---
	blobURLWithSAS := fmt.Sprintf("https://%s.blob.core.windows.net/%s?%s",
		accountName,
		containerName,
		sasToken)

	containerClient, err := container.NewClientWithNoCredential(blobURLWithSAS, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating blob client with SAS URL: %w", err)
	}

	// 1. Convert the wildcard pattern to a regular expression
	// Escape dots, replace '*' with '.*', and anchor the start/end.
	regexPattern := "^" + strings.ReplaceAll(regexp.QuoteMeta(pattern), "\\*", ".*") + "$"
	// fmt.Printf("Regex Pattern: %#v\n", regexPattern)
	// fmt.Printf("prefix path: %s \n", pathPrefix)
	nameRegex := regexp.MustCompile(regexPattern)

	// 2. Prepare listing options
	// Use the pathPrefix to efficiently filter the storage directory structure.
	listOptions := &container.ListBlobsFlatOptions{
		Prefix: &pathPrefix,
	}

	var matchingFiles []string

	// 3. Iterate through all pages of blobs
	pager := containerClient.NewListBlobsFlatPager(listOptions)
	// fmt.Printf("List Blobs: %#v\n", pager)
	for pager.More() {
		resp, err := pager.NextPage(context.Background())
		if err != nil {
			return nil, fmt.Errorf("failed to list blobs page: %w", err)
		}
		// fmt.Printf("List Blobs: %#v\n", resp)

		if resp.Segment == nil {
			return nil, fmt.Errorf("no blobs found in the container")
		}

		// 4. Check each blob name against the regex
		for _, blobItem := range resp.Segment.BlobItems {
			blobName := *blobItem.Name

			// The blobName includes the pathPrefix (e.g., "testdata/001/testSchema.testTable.000000.csv")

			// Extract just the filename from the full blob path
			// Find the index of the last separator ("/")
			lastSlash := strings.LastIndex(blobName, "/")
			filename := blobName
			if lastSlash != -1 {
				filename = blobName[lastSlash+1:]
			}

			// Apply the regex filter only to the extracted filename
			if nameRegex.MatchString(filename) {
				matchingFiles = append(matchingFiles, blobName)
			}
		}
	}

	return matchingFiles, nil
}

// NOTE: You must install the modern SDK: go get github.com/Azure/azure-sdk-for-go/sdk/storage/azblob@v1.0.0
// This function downloads a single blob to a local path.
func DownloadBlob(accountName string, sasToken string, containerName string, fileName string, localFilePath string) error {
	// --- 1. Client Setup (using SAS URL) ---
	blobURLWithSAS := fmt.Sprintf("https://%s.blob.core.windows.net?%s",
		accountName,
		sasToken)

	blobClient, err := azblob.NewClientWithNoCredential(blobURLWithSAS, nil)
	if err != nil {
		return fmt.Errorf("error creating blob client with SAS URL: %w", err)
	}

	// --- 2. File and Concurrency Setup ---
	out, err := os.Create(localFilePath)
	if err != nil {
		return fmt.Errorf("error creating local file: %w", err)
	}
	defer out.Close()

	// Define your desired concurrency level (e.g., 8 parallel chunks)
	const downloadConcurrency = 8

	// --- 3. Download with Options ---
	_, err = blobClient.DownloadFile(
		context.Background(),
		containerName,
		fileName,
		out,
		&azblob.DownloadFileOptions{
			// Specify the number of concurrent goroutines for downloading chunks
			Concurrency: downloadConcurrency,
		},
	)

	if err != nil {
		return fmt.Errorf("error downloading blob '%s': %w", fileName, err)
	}

	// fmt.Printf("Successfully downloaded %s using %d concurrent connections.\n", fileName, downloadConcurrency)
	return nil
}

// UploadBlobWithSAS uploads a local file to a specified blob path using a SAS token.
func UploadBlobWithSAS(accountName string, containerName string, fileName string, sasToken string, localFilePath string) error {

	// 1. Construct the full SAS URL for the specific blob
	// The structure is: https://<account>.blob.core.windows.net/<container>/<blob>?<SAS_TOKEN>
	blobURLWithSAS := fmt.Sprintf("https://%s.blob.core.windows.net?%s",
		accountName,
		sasToken)

	// 2. Create the blob client using the SAS URL.
	// NewClientWithNoCredential is used because the authentication is in the URL.
	blobClient, err := azblob.NewClientWithNoCredential(blobURLWithSAS, nil)
	if err != nil {
		return fmt.Errorf("error creating blob client with SAS URL: %w", err)
	}

	// --- 2. File and Concurrency Setup ---
	in, err := os.Open(localFilePath)
	if err != nil {
		return fmt.Errorf("error opening local file: %w", err)
	}
	defer in.Close()

	// Define concurrency for efficient chunked upload
	const uploadConcurrency = 5

	// 4. Upload the file using the SDK's concurrent method
	// The UploadFile method automatically handles splitting the file into blocks and uploading them in parallel.
	_, err = blobClient.UploadFile(
		context.Background(),
		containerName,
		fileName,
		in,
		&azblob.UploadFileOptions{
			Concurrency: uploadConcurrency,
		},
	)

	if err != nil {
		return fmt.Errorf("error uploading blob '%s': %w", fileName, err)
	}

	// fmt.Printf("Successfully uploaded %s (%d bytes) to Azure Blob Storage.\n", localFilePath, fileInfo.Size())
	return nil
}

func makeIDsUnique4Local(schemaName, tableName, folderPath string) ([]map[string]string, error) {
	// Get all CSV files matching the pattern
	pattern := fmt.Sprintf("%s.%s.*.csv", schemaName, tableName)
	files, err := filepath.Glob(filepath.Join(folderPath, pattern))
	if err != nil {
		return nil, fmt.Errorf("error finding CSV files: %v", err)
	}
	// fmt.Printf("Found %d CSV files matching pattern %#v\n", len(files), files)

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

	mapState := []map[string]string{}

	// Process each file
	for _, file := range files {
		if _, exists := processedFiles[file]; exists {
			continue
		}
		startTime := time.Now()
		// Get file info to determine size
		fileInfo, err := os.Stat(file)
		if err != nil {
			return nil, fmt.Errorf("error getting file info: %v", err)
		}
		theMapState := map[string]string{
			"file_name":      file,
			"file_size":      formatSize(fileInfo.Size()),
			"state":          "pending",
			"execution_time": "",
		}

		idCounter, err = makeUniqeID4File(file, idCounter)
		if err != nil {
			executionTime := time.Since(startTime)
			theMapState["execution_time"] = executionTime.String()
			theMapState["state"] = "failed"
			mapState = append(mapState, theMapState)
			return mapState, fmt.Errorf("error processing file %s: %v", file, err)
		}

		executionTime := time.Since(startTime)
		theMapState["execution_time"] = executionTime.String()
		theMapState["state"] = "completed"
		mapState = append(mapState, theMapState)

		// Save max ID to checkpoint file
		checkpointFile := filepath.Join(filepath.Dir(file), ".checkpoint")
		checkpointContent := fmt.Sprintf("%s,%d", file, idCounter)

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
			return nil, fmt.Errorf("error writing checkpoint file: %v", err)
		}
	}

	return mapState, nil
}

func makeUniqeID4File(fileName string, idCounter int64) (int64, error) {
	// Open original file
	f, err := os.Open(fileName)
	if err != nil {
		return 0, fmt.Errorf("error opening file %s: %v", fileName, err)
	}

	// Create temporary output file
	tempFile := fileName + ".tmp"
	out, err := os.Create(tempFile)
	if err != nil {
		f.Close()
		return 0, fmt.Errorf("error creating temp file: %v", err)
	}

	reader := csv.NewReader(f)
	writer := csv.NewWriter(out)

	// Read and process each row
	firstRow := true
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			f.Close()
			out.Close()
			os.Remove(tempFile)
			return 0, fmt.Errorf("error reading CSV: %v", err)
		}

		if firstRow {
			// Write header row as-is
			firstRow = false
		} else {
			// Replace ID with new unique ID
			record[0] = strconv.FormatInt(idCounter, 10)
			idCounter++
		}

		if err := writer.Write(record); err != nil {
			f.Close()
			out.Close()
			os.Remove(tempFile)
			return 0, fmt.Errorf("error writing CSV: %v", err)
		}

		// Flush writer every 100000 records to manage memory usage
		if idCounter%100000 == 0 {
			writer.Flush()
			if err := writer.Error(); err != nil {
				f.Close()
				out.Close()
				os.Remove(tempFile)
				return 0, fmt.Errorf("error flushing writer: %v", err)
			}
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		f.Close()
		out.Close()
		os.Remove(tempFile)
		return 0, fmt.Errorf("error flushing writer: %v", err)
	}

	f.Close()
	out.Close()

	// Replace original file with temporary file
	if err := os.Rename(tempFile, fileName); err != nil {
		os.Remove(tempFile)
		return 0, fmt.Errorf("error replacing original file: %v", err)
	}

	return idCounter, nil
}

func formatSize(size int64) string {
	const (
		B  = 1
		KB = 1024 * B
		MB = 1024 * KB
		GB = 1024 * MB
	)

	switch {
	case size < KB:
		return fmt.Sprintf("%d B", size)
	case size < MB:
		return fmt.Sprintf("%.2f KB", float64(size)/float64(KB))
	case size < GB:
		return fmt.Sprintf("%.2f MB", float64(size)/float64(MB))
	default:
		return fmt.Sprintf("%.2f GB", float64(size)/float64(GB))
	}
}

func printSummary(processState []map[string]string) {
	if printSummary == nil {
		return
	}
	// Create a table to display file processing information
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"File Name", "File Size", "State", "Execution Time"})

	for _, state := range processState {
		t.AppendRow(table.Row{
			state["file_name"],
			state["file_size"],
			state["state"],
			state["execution_time"],
		})
	}
	t.Render()
}
