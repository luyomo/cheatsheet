build: clean python-base lambda cf

python-base:
	mkdir python 
	pip install --target python pymysql
	pip install --target python boto3
	zip -r python_base.zip python
	aws s3 cp python_base.zip s3://jay-data/lambda/layers/

lambda:
	zip lambda_function.zip lambda_function.py
	aws s3 cp lambda_function.zip s3://jay-data/lambda/source/

cf:
	aws s3 cp template.yaml s3://jay-data/lambda/cloudformation/mysqldump-to-s3.yaml

clean:
	@if [ -d /tmp/lambda-mysqldump-to-s3  ] ; then rm -rf /tmp/lambda-mysqldump-to-s3 ; fi
	@if [ -d python  ] ; then rm -rf python ; fi
	@if [ -f python_base.zip ] ; then rm python_base.zip ; fi
	@if [ -f lambda_function.zip ] ; then rm lambda_function.zip ; fi

# layer mysqldump
# layer python-base
# lambda
