## TiDB Migration Toolkit
### Background
This toolkit is designed to facilitate database migration from a MySQL source to a TiDB destination. It addresses two primary migration scenarios: one-to-one table migration and the consolidation of multiple sharded source tables into a single destination table.

The tool automatically analyzes table structures by calculating digests based on column information. This digest-based approach allows the script to intelligently identify tables with identical layouts, even if their names differ.
- **One-to-one Migration**: For a single source table with a matching destination table of the same layout, the script generates a one-to-one mapping.
- **Table Consolidation (Sharding)**: When multiple sharded source tables have the same layout and match a single destination table, the script generates a command to merge them into a single file for import.
- **Complex Scenarios**: The script can also handle cases where both the source and destination databases have multiple tables with identical layouts but the same name. These are separated into one-to-one mappings.

## Approach
The core of the script is its ability to calculate an MD5 digest of a table's column layout and column layout with data types. It uses this digest to find tables with identical structures across the source and destination databases.
The process is as follows:
- **Analyze Source**: The script connects to the source MySQL database and calculates the digest for each table's structure.
- **Analyze Destination**: The script then connects to the TiDB destination database and performs the same analysis.
- **Generate Mappings**: The script compares the digests to create mappings:
  - One-to-one: If a single source table matches a single destination table, it's treated as a direct mapping.
  - Consolidation: If multiple source tables have the same digest and match a single destination table, they are grouped for consolidation.
- **Template-Based Command Generation**: The tool accepts a template string where users can define their desired migration command (e.g., dumpling). The script fills in the source and destination table names based on the generated mappings.

## Example Commands
- Source Analysis
This command analyzes the structure of all tables within the specified source databases and prints a summary.
```
$ md-toolkit --src-host $hostname --src-port $port --src-user $user --src-password $password --src-dbs db_00,db_01,db_02 --ops-type sourceAnalyze

Starting to analyze the source table and check the table structure 
idx: 1, md5: a3138e74a921c075aab2b2fde3c0cab4, md5 with type: 8d36d15920d80f2d76fbe59ed8b256f8, source table: "db_00.table001", dest tables: 2 
idx: 9, md5: 34dedf1dc1949790ee9b3b9546490871, md5 with type: 44c55526d3975d33acf6280c02ea677a, source table: "db_00.table002", dest tables: 2 
idx: 10, md5: 34dedf1dc1949790ee9b3b9546490871, md5 with type: 560471b0a6b2a904cb196710d137830f, source table: "db_00.table03", dest tables: 21 
idx: 19, md5: d1576899c3f5f7efbe97febdaab75a5d, md5 with type: 5df5ef12b43a588cce2e06f8e075fcfb, source table: "db_00.table04", dest tables: 2 
idx: 37, md5: d7828885c912cab588855bd1813e0c0c, md5 with type: 10e7ea2b77ec30ded46f12ed0595db20, source table: "db_00.table05", dest tables: 513 
idx: 38, md5: 4c667b6f878254fa8a45e5703ccf1d18, md5 with type: 3499eb151d3702ac2f06311ac74cf15d, source table: "db_00.table06", dest tables: 16 
idx: 39, md5: 748fab7b7bb32831bd2cbabd47a40584, md5 with type: c5472f590ac1e3ce3eab3fe2a1bdbf97, source table: "db_00.table07", dest tables: 512 
idx: 42, md5: 7e0d6b24a1ec112691f1a4f104cf58bf, md5 with type: 2b6154e13dc3ff1c2a7eb12be87948f6, source table: "db_00.table08", dest tables: 32 
idx: 0, md5: d623ba37c27286a428cb4b857a6b0f0e, md5 with type: efe3fc1b31106d4f364624dc5d1ff010, source table: "db_00.table09", dest tables: 1 
... ...

```
- Command Generation
This command generates the dumpling commands based on the table mappings and a provided template.
```
$ md-toolkit --src-host $SrcDBHost --src-port $SrcDBPort --src-user $SrcDBUser --src-password $SrcDBPassword --src-dbs db_00,db_01,messagedb_02 --dest-host $DestDBHost --dest-port $DestDBPort --dest-user $DestDBUser --dest-password $DestDBPassword --dest-dbs tidb_db --ops-type generateDumpling --template "dumpling -h \${DBHOST} -P \${DBPORT} -u \${DBUSER} -p \"\${DBPASSWORD}\" --threads 8 --tables-list '{{.SrcTable}}' --output-filename-template '{{.DestTable}}' --filetype csv -o 'azblob://exporting/merged_table_test/' --azblob.account-name $StorageAccount --azblob.sas-token \"\${SAS}\""

dumpling -h ${DBHOST} -P ${DBPORT} -u ${DBUSER} -p "${DBPASSWORD}" --threads 8 --tables-list 'db_00.table001' --output-filename-template 'tidb_db.table001.{{.Index}}' --filetype csv -o 'azblob://exporting/merged_table_test/' --azblob.account-name $StorageAccount --azblob.sas-token "${SAS}"          
dumpling -h ${DBHOST} -P ${DBPORT} -u ${DBUSER} -p "${DBPASSWORD}" --threads 8 --tables-list 'db_00.table002' --output-filename-template 'tidb_db.table002.{{.Index}}' --filetype csv -o 'azblob://exporting/merged_table_test/' --azblob.account-name $StorageAccount --azblob.sas-token "${SAS}"
...
dumpling -h ${DBHOST} -P ${DBPORT} -u ${DBUSER} -p "${DBPASSWORD}" --threads 8 --tables-list 'db_00.sharding_table_00' --output-filename-template 'tidb_db.merged_table.00001{{.Index}}' --filetype csv -o 'azblob://exporting/merged_table_test/' --azblob.account-name $StorageAccount --azblob.sas-token "${SAS}"
dumpling -h ${DBHOST} -P ${DBPORT} -u ${DBUSER} -p "${DBPASSWORD}" --threads 8 --tables-list 'db_00.sharding_table_01' --output-filename-template 'tidb_db.merged_table.00002{{.Index}}' --filetype csv -o 'azblob://exporting/merged_table_test/' --azblob.account-name $StorageAccount --azblob.sas-token "${SAS}"
dumpling -h ${DBHOST} -P ${DBPORT} -u ${DBUSER} -p "${DBPASSWORD}" --threads 8 --tables-list 'db_00.sharding_table_02' --output-filename-template 'tidb_db.merged_table.00003{{.Index}}' --filetype csv -o 'azblob://exporting/merged_table_test/' --azblob.account-name $StorageAccount --azblob.sas-token "${SAS}"
...
```
