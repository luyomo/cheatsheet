# CSV ID Generator

## Background
This tool helps generate unique IDs for CSV files while preserving the original data structure. It supports both local files and Azure Blob Storage. It's particularly useful when you need to:
- Add unique identifiers to existing CSV data
- Handle CSV files with special characters
- Process large datasets while maintaining data integrity
- Work with Azure Blob Storage for cloud-based data processing

## How to Use

### Local Files
1. Download the latest release from the Releases page
2. Run the executable with your CSV file:
   ```
   $ gen-unique-id -s local --table testTable --schema testSchema --path "testdata/001"
   +----------------------------------------------+-----------+-----------+----------------+
   | FILE NAME                                    | FILE SIZE | STATE     | EXECUTION TIME |
   +----------------------------------------------+-----------+-----------+----------------+
   | testdata/001/testSchema.testTable.000000.csv | 2.11 KB   | completed | 157.743µs      |
   | testdata/001/testSchema.testTable.000001.csv | 2.09 KB   | completed | 92.368µs       |
   +----------------------------------------------+-----------+-----------+----------------+
   ```

### Azure Blob Storage
1. Ensure you have a valid SAS token and Azure storage account
2. Upload your CSV to Azure Blob Storage:
   ```
   az storage blob upload \
       --account-name <storage-account> \
       --container-name <container> \
       --file <local-file-path> \
       --name <blob-path> \
       --sas-token "<sas-token>"
   ```
3. Run the tool with Azure configuration:
   ```
   $ gen-unique-id -s azure \
       --sas-token <sas-token> \
       --storage-name <storage-account> \
       --container <container> \
       --table <table-name> \
       --schema <schema-name> \
       --path <path>
   +---------------------------------------------------+-----------+-----------+----------------+
   | FILE NAME                                         | FILE SIZE | STATE     | EXECUTION TIME |
   +---------------------------------------------------+-----------+-----------+----------------+
   | merged_table_test/testSchema.testTable.000000.csv | 1.52 KB   | completed | 27.115097ms    |
   | merged_table_test/testSchema.testTable.000001.csv | 1.60 KB   | completed | 27.850628ms    |
   +---------------------------------------------------+-----------+-----------+----------------+ 
   ```

## Features
The tool will:
- Read your input CSV (local or from Azure Blob Storage)
- Generate unique IDs
- Create a new CSV with IDs while preserving original data
- Handle special characters automatically
- Store processing state in .checkpoint file for resuming operations

## Special Character Support
The tool handles various special characters including:
- Commas (,)
- Double quotes (")
- New lines in data fields

## Resumable Operations
Processing can be resumed from the last checkpoint in case of interruption, making it suitable for large datasets.
