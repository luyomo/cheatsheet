import azure.functions as func
import logging
import json
import os
from datetime import datetime
import requests
from requests.auth import HTTPDigestAuth
from typing import Optional, Dict, Any

app = func.FunctionApp()

@app.function_name(name="ResumeTiDBScheduled")
@app.schedule(
    schedule=os.environ.get("RESUME_SCHEDULE_CRON", "0 0 7 * * *"), 
    arg_name="mytimer", 
    run_on_startup=False
)  
def resume_tidb_scheduled(mytimer: func.TimerRequest) -> None:
    disableSchedule  = os.environ.get("DISABLE_SCHEDULE", "false")
    current_cron_schedule = os.environ.get("RESUME_SCHEDULE_CRON", "N/A")
    cluster_id  = os.environ.get("TIDB_CLOUD_CLUSTER_ID", "N/A")
    public_key  = os.environ.get("TIDB_CLOUD_PUBLIC_KEY", "N/A")
    private_key = os.environ.get("TIDB_CLOUD_PRIVATE_KEY", "N/A")
    logging.info(f'Trigger the TiDB pause. Plan: {current_cron_schedule}, Timestamp: {datetime.now().isoformat()}')

    url = f"https://dedicated.tidbapi.com/v1beta1/clusters/{cluster_id}:resumeCluster"
    if disableSchedule == "true":
        return

    try:
        res = get_cluster_status(cluster_id, public_key, private_key)
        logging.info(f"Cluster status: {res['state']}")
        if res['state'] == "ACTIVE": 
            return

        if res['state'] == "RESUMING": 
            return

        response = requests.post(
            url,
            auth=HTTPDigestAuth(public_key, private_key),
            timeout=30
        )
        if response.status_code == 200:
            return
        else:
            logging.error(f"status code: {response.status_code}, message: {response.text}")
            return {
                "success": False,
                "status_code": response.status_code,
                "message": response.text
            }
    except Exception as e:
        logging.error(f"status code: -1, message: {str(e)}")
        return {
            "success": False,
            "status_code": -1,
            "message": str(e)
        }

@app.function_name(name="PauseTiDBScheduled")
@app.schedule(
    schedule=os.environ.get("PAUSE_SCHEDULE_CRON", "0 0 23 * * *"), 
    arg_name="mytimer", 
    run_on_startup=False
)  
def pause_tidb_scheduled(mytimer: func.TimerRequest) -> None:
    disableSchedule  = os.environ.get("DISABLE_SCHEDULE", "false")
    current_cron_schedule = os.environ.get("PAUSE_SCHEDULE_CRON", "N/A")
    cluster_id  = os.environ.get("TIDB_CLOUD_CLUSTER_ID", "N/A")
    public_key  = os.environ.get("TIDB_CLOUD_PUBLIC_KEY", "N/A")
    private_key = os.environ.get("TIDB_CLOUD_PRIVATE_KEY", "N/A")
    logging.info(f'Trigger the TiDB pause. Plan: {current_cron_schedule}, Timestamp: {datetime.now().isoformat()}')


    url = f"https://dedicated.tidbapi.com/v1beta1/clusters/{cluster_id}:pauseCluster"
    if disableSchedule == "true":
        return

    try:
        res = get_cluster_status(cluster_id, public_key, private_key)
        logging.info(f"--- response: {res}")
        logging.info(f"--- response: {res['state']}")
        if res['state'] == "PAUSED": 
            return

        if res['state'] == "PAUSING": 
            return

        response = requests.post(
            url,
            auth=HTTPDigestAuth(public_key, private_key),
            timeout=30
        )
        if response.status_code == 200:
            return
        else:
            return {
                "success": False,
                "status_code": response.status_code,
                "message": response.text
            }
    except Exception as e:
        logging.info(f"Error: {e}")
        return {
            "success": False,
            "status_code": -1,
            "message": str(e)
        }

def get_cluster_status(cluster_id: str, public_key: str, private_key: str) -> Optional[Dict[str, Any]]:
    url = f"https://dedicated.tidbapi.com/v1beta1/clusters/{cluster_id}"
    
    try:
        response = requests.get(
            url,
            auth=requests.auth.HTTPDigestAuth(public_key, private_key),
            headers={'Accept': 'application/json'},
            timeout=15
        )
        response.raise_for_status()
        return response.json()
        
    except requests.exceptions.RequestException as e:
        logging.error(f"Failed to get the cluster status: {e}")
        return None
