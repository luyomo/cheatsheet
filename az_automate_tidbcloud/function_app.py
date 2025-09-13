import azure.functions as func
import logging
import json
import os
from datetime import datetime
import requests
from requests.auth import HTTPDigestAuth

app = func.FunctionApp()

@app.function_name(name="HttpTrigger1")
@app.route(route="hello", auth_level=func.AuthLevel.ANONYMOUS)
def hello_world(req: func.HttpRequest) -> func.HttpResponse:
    logging.info('Python HTTP trigger function processed a request.')

    # Get name from query string or request body
    name = req.params.get('name')
    if not name:
        try:
            req_body = req.get_json()
            name = req_body.get('name')
        except ValueError:
            pass

    # Personalized response
    if name:
        message = f"Hello, {name}! This Azure Function executed successfully at {datetime.utcnow().isoformat()}Z"
        status_code = 200
    else:
        message = "Please pass a name on the query string or in the request body to get a personalized greeting."
        status_code = 400

    # Return HTTP response
    return func.HttpResponse(
        json.dumps({
            "message": message,
            "timestamp": datetime.utcnow().isoformat() + "Z",
            "success": True if name else False
        }),
        status_code=status_code,
        mimetype="application/json"
    )

@app.function_name(name="ResumeTiDBScheduled")
@app.schedule(
    schedule=os.environ.get("RESUME_SCHEDULE_CRON", "0 0 7 * * *"), 
    arg_name="mytimer", 
    run_on_startup=False
)  
def resume_tidb_scheduled(mytimer: func.TimerRequest) -> None:
    current_cron_schedule = os.environ.get("RESUME_SCHEDULE_CRON", "N/A")
    cluster_id  = os.environ.get("TIDB_CLOUD_CLUSTER_ID", "N/A")
    public_key  = os.environ.get("TIDB_CLOUD_PUBLIC_KEY", "N/A")
    private_key = os.environ.get("TIDB_CLOUD_PRIVATE_KEY", "N/A")
    logging.info(f'Trigger the TiDB pause. Plan: {current_cron_schedule}, Timestamp: {datetime.now().isoformat()}')

    url = f"https://dedicated.tidbapi.com/v1beta1/clusters/{cluster_id}:resumeCluster"

    try:
        response = requests.post(
            url,
            auth=HTTPDigestAuth(public_key, private_key),
            timeout=30
        )
        if response.status_code == 200:
            return {
                "success": True,
                "status_code": response.status_code,
                "message": response.text
            }
        else:
            return {
                "success": False,
                "status_code": response.status_code,
                "message": response.text
            }
    except Exception as e:
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
def resume_tidb_scheduled(mytimer: func.TimerRequest) -> None:
    current_cron_schedule = os.environ.get("PAUSE_SCHEDULE_CRON", "N/A")
    cluster_id  = os.environ.get("TIDB_CLOUD_CLUSTER_ID", "N/A")
    public_key  = os.environ.get("TIDB_CLOUD_PUBLIC_KEY", "N/A")
    private_key = os.environ.get("TIDB_CLOUD_PRIVATE_KEY", "N/A")
    logging.info(f'Trigger the TiDB pause. Plan: {current_cron_schedule}, Timestamp: {datetime.now().isoformat()}')

    url = f"https://dedicated.tidbapi.com/v1beta1/clusters/{cluster_id}:pauseCluster"

    try:
        response = requests.post(
            url,
            auth=HTTPDigestAuth(public_key, private_key),
            timeout=30
        )
        if response.status_code == 200:
            return {
                "success": True,
                "status_code": response.status_code,
                "message": response.text
            }
        else:
            return {
                "success": False,
                "status_code": response.status_code,
                "message": response.text
            }
    except Exception as e:
        return {
            "success": False,
            "status_code": -1,
            "message": str(e)
        }
