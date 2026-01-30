# Migration Data Toolkit (md-toolkit) - DM Module

The `DM` (Data Migration) module of `md-toolkit` automates the creation of source and task configuration files for **TiDB Data Migration (DM)**. By leveraging the same centralized configuration used for `dumpling` and `sync-diff-inspector`, it ensures that your data replication pipeline is perfectly aligned with your export and verification logic.

## Background

Setting up TiDB DM for sharded MySQL environments is traditionally a multi-step, manual process:

* **Redundancy:** Defining the same source connection details across multiple YAML files.
* **Complex Routing:** Manually writing regular expressions for `table-rule` to consolidate hundreds of shards.

This module automates these steps, generating both the **Source Configuration** and the **Migration Task Configuration** in seconds.

---

## Key Features

### 1. Dual-Output Generation

The tool generates two distinct sets of files required by `dmctl`:

* **Source Files:** Connection profiles for each upstream MySQL instance.
* **Task Files:** The logic for the migration task (Incremental), including sharding rules.

### 2. Intelligent Shard Mapping

Using DeepSeek LLM, the tool identifies naming patterns across your sharded instances (e.g., `db_01.user_01` through `db_99.user_99`) and collapses them into concise `route-rules` using regex.

### 3. Automated Metadata Injection

If the migration requires Pattern 3 logic (Primary Key conflict resolution), the DM module automatically configures the task to handle the additional `c_schema` and `c_table` columns, ensuring the replication stream matches the target TiDB schema.

---

## Specifications

### Command

```bash
./bin/md-toolkit --config config/config.yaml --ops-type generateDMConfig --llm deepseek
```
If the command does not include --llm deepseek, it will skip the regret generation. Use the ---------- todo --------- in the output. After the config file is generated, you need to replace it manually.

### Pattern Mapping

| Migration Pattern | DM Strategy | Result |
| --- | --- | --- |
| **Pattern 1 & 2** | Standard Routing | Simple 1:1 or N:1 mapping. |
| **Pattern 3** | Column Mapping | Injects metadata to resolve PK conflicts during replication. |

---

## Usage Example

### 1. Input Configuration (`config/config.yaml`)

The tool utilizes the same "Single Source of Truth" configuration file:

```yaml
SourceDB:
  - Name: instance01
    Host: 10.0.1.5
    Port: 3306
    # ... credentials ...
    DBs: [db_00, db_01]
  - Name: instance02
    Host: 10.0.1.6
    Port: 3306
    # ... credentials ...
    DBs: [db_08, db_09]

DestDB:
  Name: targetDB
  # ... TiDB credentials ...

```

### 2. Generated Output

#### A. Source Configuration (`source_instance01.yaml`)

```yaml
# Auto-generated Source Config
source-id: "instance01"
from:
  host: "10.0.1.5"
  port: 3306
  user: "dmuser"
  password: "..."

```

#### B. Task Configuration (`task_migration.yaml`)

```yaml
name: "shard_consolidation_task"
task-mode: "Incremental"
is-sharding: true

target-database:
  host: "10.0.1.4"
  port: 4000
  user: "root"

routes:
  shard-route-rule:
    schema-pattern: "db_*"
    target-schema: "db"

```