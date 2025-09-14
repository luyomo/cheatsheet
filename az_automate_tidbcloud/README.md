## TiDB Cloud Cluster Scheduler
This script contains an Azure Functions application to automatically pause and resume a dedicated TiDB Cloud cluster on a predefined schedule. This is useful for managing costs by pausing the cluster during off-hours, while ensuring it is available for business hours.

## Background
The script uses two timer-triggered Azure Functions:
- ResumeTiDBScheduled: Resumes the cluster at a specified time.
- PauseTiDBScheduled: Pauses the cluster at a specified time.
The functions communicate with the TiDB Cloud API using an authenticated HTTP request to check the cluster's status and then send a pause or resume command.
Please refer to [TiDB Cloud API](https://docs.pingcap.com/tidbcloud/api/v1beta1/dedicated/#tag/Cluster/operation/ClusterService_PauseCluster)

## Prerequisites
- Python 3.12 or a later version.
- Azure Functions Core Tools.
- Azure CLI.

## configuration
The functions use environment variables for configuration. You can set these in your local.settings.json for local development or in the Azure portal for production.
``` local.settings.json
{
  "IsEncrypted": false,
  "Values": {
    "AzureWebJobsStorage": "UseDevelopmentStorage=true",
    "FUNCTIONS_WORKER_RUNTIME": "python",
    "TIDB_CLOUD_CLUSTER_ID": "<your-cluster-id>",
    "TIDB_CLOUD_PUBLIC_KEY": "<your-tidb-public-key>",
    "TIDB_CLOUD_PRIVATE_KEY": "<your-tidb-private-key>",
    "PAUSE_SCHEDULE_CRON": "0 0 23 * * *",
    "RESUME_SCHEDULE_CRON": "0 0 7 * * *",
    "DISABLE_SCHEDULE": "false"
  }
}

```

Please refer to [API KEY](https://docs.pingcap.com/tidbcloud/api/v1beta1/dedicated/#section/Get-Started/Prerequisites) to get the cluster api key.
Environment Variables
- TIDB_CLOUD_CLUSTER_ID: The unique ID of your TiDB Cloud cluster.
- TIDB_CLOUD_PUBLIC_KEY: Your TiDB Cloud API Public Key.
- TIDB_CLOUD_PRIVATE_KEY: Your TiDB Cloud API Private Key. This is a secret and should be stored securely, ideally in an Azure Key Vault.
- PAUSE_SCHEDULE_CRON: A CRON expression for when to pause the cluster (e.g., 0 0 23 * * * for 11:00 PM every day).
- RESUME_SCHEDULE_CRON: A CRON expression for when to resume the cluster (e.g., 0 0 7 * * * for 7:00 AM every day).
- DISABLE_SCHEDULE: Set to true to temporarily disable the scheduled functions without removing them.

## Deployment
To deploy your function app to Azure, you can use the Azure CLI.
- Log in to Azure:
```
az login
```
- Deploy the project:
```
$ az functionapp create --name $APP_NAME    \
      --storage-account $STORAGE_NAME       \
      --consumption-plan-location $LOCATION \
      --resource-group $RESOURCE_GROUP      \
      --os-type Linux                       \
      --runtime python                      \
      --runtime-version $PYTHON_VERSION     \
      --functions-version 4 
$ func azure functionapp publish $APP_NAME
```
Important: After deployment, remember to add your environment variables to the Configuration section of your Function App in the Azure Portal.
