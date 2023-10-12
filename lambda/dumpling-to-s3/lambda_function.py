import sys
import logging
import pymysql
import json
import os
import boto3
import csv
import subprocess

# rds settings
DATABASE_LIST = ["test"]

TEMP_BASE_DIR = "/tmp"
PATH_TEMPLATE = '{base_dir}/{db_host}/{db_name}'

logger = logging.getLogger()
logger.setLevel(logging.INFO)

class MySQLDump:
    def __init__(self, rdsHost: str, rdsPort: int, rdsUser: str, rdsPassword: str, s3Bucket: str, s3Key: str, backupDBs: list = None ):
        self.RDSHost     = rdsHost
        self.RDSPort     = rdsPort
        self.RDSUser     = rdsUser
        self.RDSPassword = rdsPassword
        self.S3Bucket    = s3Bucket
        self.S3Key       = s3Key
        self.BackupDBs   = backupDBs

        # create the database connection outside of the handler to allow connections to be
        # re-used by subsequent function invocations.
        try:
            self.conn = pymysql.connect(host=self.RDSHost, port=self.RDSPort, user=self.RDSUser, passwd=self.RDSPassword, db="mysql", connect_timeout=5)
        except pymysql.MySQLError as e:
            logger.error("ERROR: Unexpected error: Could not connect to MySQL instance.")
            logger.error(e)
            sys.exit()

        logger.info("SUCCESS: Connection to RDS MySQL instance succeeded")

        if self.BackupDBs is None:
            self.BackupDBs = self.fetchAllDBs()

    def upload_s3(self, fileName: str):
        s3 = boto3.client('s3')
        for subdir, dirs, files in os.walk(fileName):
            for file in files:
                full_path = os.path.join(subdir, file)
                logger.info(f"The path: {full_path} -> {self.S3Bucket} / {self.S3Key}/{os.path.basename(full_path)} ")
                response = s3.upload_file(full_path,  self.S3Bucket, f"{self.S3Key}/{os.path.basename(full_path)}")

                # with open(full_path, 'rb') as data:
                #     bucket.put_object(Key=bucketFolderName, Body=data)

        # response = s3.upload_file(fileName,  self.S3Bucket, f"{self.S3Key}/{os.path.basename(fileName)}")
        # logger.info(f"upload file response: {response}")

    def Backup(self) -> bool:
        logger.info("Starting to backup the database")
        for db in self.BackupDBs:
            localPath = self.save_file_to_local(db, False)
            logger.info(f"The exported file: {localPath}")
            self.upload_s3(localPath)

    def fetchAllDBs(self) -> list:
        targetDBs = []
        with self.conn.cursor() as cur:
            cur.execute("select distinct table_schema from information_schema.tables where table_schema not in ('information_schema', 'sys', 'mysql', 'performance_schema')")
            logger.info("The following items have been added to the database:")
            for row in cur:
                targetDBs.append(row[0])

        logger.info(f"All the db are: {targetDBs}")
        return targetDBs

    def save_file_to_local(self, dbName: str, compress: bool=True):
        # exp = f'{dbName}.sql.gz' if compress else f'{dbName}.ddl.sql'

        target_db_string = f'-h {self.RDSHost} --port {self.RDSPort} -u {self.RDSUser} -p{self.RDSPassword}'

        target_local_path = PATH_TEMPLATE.format(
            base_dir=TEMP_BASE_DIR,
            db_host=self.RDSHost,
            db_name=dbName
            # file=f'{exp}'
        )
        # os.makedirs(os.path.dirname(target_local_path), exist_ok=True)  
        os.makedirs(target_local_path, exist_ok=True)

    # mysqldump -d -h jay-labmda.cluster-cxmxisy1o2a2.us-east-1.rds.amazonaws.com -u admin -p1234Abcd --database test

        mysqldump_string = f'/opt/bin/dumpling -d --filetype sql -o {target_local_path} {target_db_string}'
        # command = f'{mysqldump_string} | gzip -9 > {target_local_path};' if compress else f'{mysqldump_string} > {target_local_path};'

        logger.info(f"command: <{mysqldump_string}>")

        response = subprocess.run(mysqldump_string, shell=True, capture_output=True)
        if response.returncode != 0:
            logger.info(f"response: {response.stderr}")

        return target_local_path


def lambda_handler(event, context):
    """
    This function creates a new RDS database table and writes records to it
    """

    msg = ""
    if "RDSConn" not in event or "rds_host" not in event["RDSConn"] or "rds_port" not in event["RDSConn"] or "rds_user" not in event["RDSConn"] or "rds_password" not in event["RDSConn"]:
        msg = "Please input the parameters like (eg: {'rds_host': 'localhost', 'rds_port': 3306, 'rds_user': 'rds_user', 'rds_password': 'password'})"
    if msg != "":
        return msg

    if "S3" not in event or "BucketName" not in event["S3"] or "S3Key" not in event["S3"]:
        msg = "Please inout the parameters like {'S3': {'BucketName': 'bucket name', 'S3Key': 's3 key'}}"

    if msg != "":
        return msg

    if "TargetDBs" not in event:
        mysqlDump = MySQLDump(event["RDSConn"]["rds_host"], event["RDSConn"]["rds_port"], event["RDSConn"]["rds_user"], event["RDSConn"]["rds_password"], event["S3"]["BucketName"], event["S3"]["S3Key"])
    else:
        mysqlDump = MySQLDump(event["RDSConn"]["rds_host"], event["RDSConn"]["rds_port"], event["RDSConn"]["rds_user"], event["RDSConn"]["rds_password"], event["S3"]["BucketName"], event["S3"]["S3Key"], event["TargetDBs"])

    mysqlDump.Backup()

    return "Successfully dump the ddl"
