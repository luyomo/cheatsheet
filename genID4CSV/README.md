# CSV ID Generator

## Background
This tool helps generate unique IDs for CSV files while preserving the original data structure. It's particularly useful when you need to:
- Add unique identifiers to existing CSV data
- Handle CSV files with special characters
- Process large datasets while maintaining data integrity

## How to Use
### Execution for local files
1. Download the latest release from the Releases page
2. Run the executable with your CSV file:
   ```
   ./bin/gen-unique-id -s local --table testTable --schema testSchema --path "testdata/001"
   ```
3. The tool will:
   - Read your input CSV
   - Generate unique IDs
   - Create a new CSV with IDs while preserving original data
   - Handle special characters automatically
   - Store processing state in .checkpoint file for resuming operations

## Test Cases

### Special Characters Handling
The tool successfully handles various special characters:

1. Comma (,), Double quore(") and New line in data:
2. Resume the generation of IDs from the last generated ID.


az storage blob upload \
    --account-name jaytest001 \
    --container-name tidbdataimport \
    --file testdata/001/testSchema.testTable.000000.csv \
    --name merged_table_test/test001.csv \
    --sas-token "${SAS}"

./bin/gen-unique-id -s azure --sas-token $SAS --storage-name jaytest001 --container tidbdataimport --table testTable --schema testSchema --path "testdata/001"
