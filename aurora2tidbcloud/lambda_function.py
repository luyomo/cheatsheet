import sys
import logging
import pymysql
import json
import os
import boto3
import csv
import subprocess

# rds settings
user_name = os.environ['USER_NAME']
password  = os.environ['PASSWORD']
rds_host  = os.environ['RDS_HOST']
db_name   = os.environ['DB_NAME']
DATABASE_LIST = ["test"]
TEMP_BASE_DIR = "/tmp"
PATH_TEMPLATE = '{base_dir}/{db_host}/{db_name}/{file}'

logger = logging.getLogger()
logger.setLevel(logging.INFO)

# create the database connection outside of the handler to allow connections to be
# re-used by subsequent function invocations.
try:
    conn = pymysql.connect(host=rds_host, user=user_name, passwd=password, db=db_name, connect_timeout=5)
except pymysql.MySQLError as e:
    logger.error("ERROR: Unexpected error: Could not connect to MySQL instance.")
    logger.error(e)
    sys.exit()

logger.info("SUCCESS: Connection to RDS MySQL instance succeeded")

def upload_s3(fileName: str):
    s3 = boto3.client('s3')
    response = s3.upload_file(fileName,  "jay-data", f"lambda/data/{os.path.basename(fileName)}")
    logger.info(f"upload file response: {response}")

def backup() -> bool:
    logger.info("Starting to backup the database")
    for db in DATABASE_LIST:
        localPath = save_file_to_local(db, False)
        logger.info(f"The exported file: {localPath}")
        upload_s3(localPath)
    
def save_file_to_local(db: dict, compress: bool=True):
    exp = 'sql.gz' if compress else 'ddl.sql'

    target_db_string = f'-h {rds_host} --port 3306 -u {user_name} -p{password} {db_name}'

    target_local_path = PATH_TEMPLATE.format(
        base_dir=TEMP_BASE_DIR,
        db_host=rds_host,
        db_name="test",
        file=f'{exp}'
    )
    os.makedirs(os.path.dirname(target_local_path), exist_ok=True)

    mysqldump_string = f'/opt/bin/mysqldump --no-autocommit=1 --single-transaction=1 --extended-insert=1 {target_db_string}'
    command = f'{mysqldump_string} | gzip -9 > {target_local_path};' if compress else f'{mysqldump_string} > {target_local_path};'

    logger.info(f"command: <{command}>")

    response = subprocess.run(command, shell=True, capture_output=True)
    if response.returncode != 0:
        logger.info(f"response: {response.stderr}")

    return target_local_path

def lambda_handler(event, context):
    """
    This function creates a new RDS database table and writes records to it
    """

    backup()

    return "test"
#    message = event['Records'][0]['body']
#    data = json.loads(message)
#    CustID = data['CustID']
#    Name = data['Name']
#
#    upload_s3()
#
#    item_count = 0
#    sql_string = f"insert into Customer (CustID, Name) values({CustID}, '{Name}')"
#
#    with conn.cursor() as cur:
#        cur.execute("create table if not exists Customer ( CustID  int NOT NULL, Name varchar(255) NOT NULL, PRIMARY KEY (CustID))")
#        cur.execute(sql_string)
#        conn.commit()
#        cur.execute("select * from Customer")
#        logger.info("The following items have been added to the database:")
#        for row in cur:
#            item_count += 1
#            logger.info(row)
#    conn.commit()
#
#    return "Added %d items to RDS MySQL table" %(item_count)
