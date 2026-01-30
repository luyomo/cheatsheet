# Migration Data Toolkit (md-toolkit)

`md-toolkit` is a specialized utility designed to streamline data migration from **MySQL** to **TiDB**. It automates the generation of `dumpling` export commands by analyzing source/destination schemas and applying one of three distinct migration patterns to handle consolidation and primary key conflicts.

## Background

In large-scale sharding architectures, a single logical table may be spread across hundreds of physical databases and multiple MySQL instances. Manually writing export scripts for each shard is:

* Error-prone: High risk of syntax errors or naming conflicts.
* Inscalable: Impossible to maintain when dealing with hundreds or thousands of tables.
* Complex: Handling Primary Key (PK) conflicts across shards requires custom SQL transformations.

This tool automates the discovery and command generation, allowing users to manage thousands of table migrations via a single configuration file.

## Migration Patterns

The tool categorizes migrations into three logic patterns. The application dynamically replaces the `{{.SourceData}}` placeholder in your command template based on these patterns.

### 1. One-to-One (Simple Mapping)

* **Scenario:** One MySQL source table maps to one TiDB destination table.
* **Logic:** Schema or table names may change, but the data structure remains identical.
* **SourceData Rendering:** Uses `--tables-list`.

### 2. Multiple-to-One (No Key Conflict)

* **Scenario:** Multiple source tables (shards) are consolidated into a single TiDB table.
* **Logic:** Primary keys across shards are unique. To prevent file system naming conflicts during export, the tool generates non-conflicting filenames using a `{seq}{index}` format.
* **SourceData Rendering:** Uses `--tables-list`.

### 3. Multiple-to-One (With Key Conflict)

* **Scenario:** Multiple source tables are consolidated, but Primary Keys (PK) overlap between shards.
* **Logic:** The tool injects 1–3 additional columns (`c_schema`, `c_table`, etc.) into the export stream to ensure uniqueness in the target table.
* **SourceData Rendering:** Uses a `-S` (SQL Select) statement instead of a table list to perform the column injection.

---

## Specifications

### Command Template

The tool uses a flexible template defined in your config. The core logic revolves around the `{{.SourceData}}` and `{{.DestTable}}` variables:

```bash
dumpling -h ${DBHOST} -P ${DBPORT} -u ${DBUSER} -p "${DBPASSWORD}" --threads 1 {{.SourceData}} --output-filename-template '{{.DestTable}}' --filetype csv -o "${DUMPLING_OUTPUT}"

```

### Pattern Comparison

| Feature | Pattern 1 & 2 | Pattern 3 (Conflict Resolution) |
| --- | --- | --- |
| **Method** | Table List | SQL Select Query |
| **Logic** | `--tables-list '{{.SrcTable}}'` | `-S "SELECT *,'{{.SrcSchemaName}}' as c_schema... FROM {{.SrcTable}}"` |
| **Schema Change** | No | Added metadata columns |
| **Use Case** | Standard Migration | Sharded data with overlapping IDs |

---

## Usage Example

### 1. Configuration (`config/config-special.yaml`)

Define your source instances and your target TiDB cluster. Note how the `Template` includes the variables the tool will populate.

```yaml
SourceDB:
  - Name: instance01
    Host: 10.0.1.5
    Port: 3306
    User: dmuser
    Password: 1234Abcd
    DBs: [sourcedb_00, sourcedb_01]
  - Name: instance02
    Host: 10.0.1.6
    Port: 3306
    User: dmuser
    Password: 1234Abcd
    DBs: [sourcedb_08, sourcedb_09]

DestDB:
  Name: targetDB
  Host: 10.0.1.4
  Port: 4000
  User: root
  Password: 1234Abcd
  DBs: [targetdb]

Template: "dumpling -h ${DBHOST} -P ${DBPORT} -u ${DBUSER} -p \"${DBPASSWORD}\" --threads 1 {{.SourceData}} --output-filename-template '{{.DestTable}}' --filetype csv -o \"${DUMPLING_OUTPUT}\""

```

### 2. Execution

Run the toolkit to generate the required Dumpling scripts:

```bash
./bin/md-toolkit --config config/config.yaml --ops-type generateDumpling --llm deepseek

```

### 3. Output Example (Pattern 3)

If the tool detects a key conflict, it will generate a command similar to:

```bash
dumpling -h ${DBHOST} -P ${DBPORT} -u ${DBUSER} -p "${DBPASSWORD}" \
--threads 1 \
-S "SELECT *, 'sourcedb_00' as c_schema, 'users' as c_table FROM sourcedb_00.users" \
--output-filename-template 'targetdb.users.001' \
--filetype csv -o "/export/data"
...
...
```

