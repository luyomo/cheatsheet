build: clean python-base lambda cf

python-base:
	mkdir python 
	pip install --target python pymysql
	zip -r pymysql.zip python
	aws s3 cp pymysql.zip s3://jay-data/lambda/layers/

lambda:
	zip mysqlBinlogInfo.zip lambda_function.py
	aws s3 cp  mysqlBinlogInfo.zip s3://jay-data/lambda/source/

cf:
	aws s3 cp template.yaml s3://jay-data/lambda/cloudformation/mysqlBinglogInfo.yaml

clean:
	@if [ -d python  ] ; then rm -rf python ; fi
	@if [ -f mysqlBinlogInfo.zip ] ; then rm mysqlBinlogInfo.zip ; fi
	@if [ -f pymysql.zip ] ; then rm pymysql.zip ; fi
