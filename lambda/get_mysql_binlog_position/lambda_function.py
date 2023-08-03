import sys
import logging
import pymysql
import json
import os
import boto3
import csv
import subprocess

# rds settings

TEMP_BASE_DIR = "/tmp"
logger = logging.getLogger()
logger.setLevel(logging.INFO)

class GetMySQLBinlogPos:
    def __init__(self, rdsHost: str, rdsPort: int, rdsUser: str, rdsPassword: str ):
        self.RDSHost     = rdsHost
        self.RDSPort     = rdsPort
        self.RDSUser     = rdsUser
        self.RDSPassword = rdsPassword

        # create the database connection outside of the handler to allow connections to be
        # re-used by subsequent function invocations.
        try:
            self.conn = pymysql.connect(host=self.RDSHost, port=self.RDSPort, user=self.RDSUser, passwd=self.RDSPassword, db="mysql", connect_timeout=5)
        except pymysql.MySQLError as e:
            logger.error("ERROR: Unexpected error: Could not connect to MySQL instance.")
            logger.error(e)
            sys.exit()
        
        logger.info("SUCCESS: Connection to RDS MySQL instance succeeded")


    def Execute(self) -> tuple[str, str, str, int]:
        logger.info("Starting to check binlog format")
        logBin = self.fetchLogBin()
        logger.info(f"The log bin is {logBin}")
        if logBin != "ON":
            return logBin, "", "", 0

        binlogFormat = self.fetchBinglogFormat()
        logger.info(f"The binlog format is {binlogFormat}")
        if binlogFormat != "ROW":
            return logBin, binlogFormat, "", 0

        binlogFile, binlogPos = self.fetchBinglogPosition()
        logger.info("Fetching binlog position")
        return logBin, binlogFormat, binlogFile, binlogPos

    def fetchBinglogFormat(self) -> str:
        binlogFormat = ""
        try:
            with self.conn.cursor() as cur:
                cur.execute("select VARIABLE_VALUE from information_schema.GLOBAL_VARIABLES where VARIABLE_NAME like 'binlog_format'")
                for row in cur:
                    binlogFormat = row[0]
        except pymysql.MySQLError as e:
            logger.error("ERROR: Unexpected error: Failed to fetch binlog format.")
            logger.error(e)
            sys.exit()

        return binlogFormat

    def fetchLogBin(self) -> str:
        logBin= ""
        try:
            with self.conn.cursor() as cur:
                cur.execute("select VARIABLE_VALUE from information_schema.GLOBAL_VARIABLES where VARIABLE_NAME like 'log_bin'")
                for row in cur:
                    logBin = row[0]
        except pymysql.MySQLError as e:
            logger.error("ERROR: Unexpected error: Failed to fetch binlog format.")
            logger.error(e)
            sys.exit()

        return logBin

    def fetchBinglogPosition(self) -> tuple[str, int]:
        binlogFile = ""
        binlogPos = 0
        
        try:
            with self.conn.cursor() as cur:
                cur.execute(" show master status")
                for row in cur:
                    binlogFile = row[0]
                    binlogPos = row[1]
        except pymysql.MySQLError as e:
            logger.error("ERROR: Unexpected error: Failed to fetch binlog format.")
            logger.error(e)
            sys.exit()

        return binlogFile, binlogPos

def lambda_handler(event, context):
    """
    This function creates a new RDS database table and writes records to it
    """

    msg = ""
    if "RDSConn" not in event or "rds_host" not in event["RDSConn"] or "rds_port" not in event["RDSConn"] or "rds_user" not in event["RDSConn"] or "rds_password" not in event["RDSConn"]:
        msg = "Please input the parameters like (eg: {'rds_host': 'localhost', 'rds_port': 3306, 'rds_user': 'rds_user', 'rds_password': 'password'})"
    if msg != "":
        return msg

    binlogFormat = GetMySQLBinlogPos(event["RDSConn"]["rds_host"], event["RDSConn"]["rds_port"], event["RDSConn"]["rds_user"], event["RDSConn"]["rds_password"])

    return binlogFormat.Execute()

