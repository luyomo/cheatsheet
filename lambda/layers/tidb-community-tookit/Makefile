build: clean mysqldump python-base lambda cf

dumpling: base
	@if [ ! -f /tmp/tidb-community-toolkit-v7.1.0-linux-amd64/bin/dumpling ] ; then tar xvf /tmp/tidb-community-toolkit-v7.1.0-linux-amd64/dumpling-v7.1.0-linux-amd64.tar.gz -C /tmp/tidb-community-toolkit-v7.1.0-linux-amd64/bin ; fi
	@if [ ! -f /tmp/tidb-community-toolkit-v7.1.0-linux-amd64/dumpling-v7.1.0-linux-amd64.zip ] ; then cd /tmp/tidb-community-toolkit-v7.1.0-linux-amd64; zip -r dumpling-v7.1.0-linux-amd64.zip bin ; fi
	aws s3 cp /tmp/tidb-community-toolkit-v7.1.0-linux-amd64/dumpling-v7.1.0-linux-amd64.zip s3://jay-data/lambda/layers/

base:
	@if [ ! -f /tmp/tidb-community-toolkit-v7.1.0-linux-amd64.tar.gz ] ; then wget https://download.pingcap.org/tidb-community-toolkit-v7.1.0-linux-amd64.tar.gz -P /tmp ; fi
	@if [ ! -d /tmp/tidb-community-toolkit-v7.1.0-linux-amd64 ] ; then tar xvf /tmp/tidb-community-toolkit-v7.1.0-linux-amd64.tar.gz -C /tmp ; fi
	@if [ ! -d /tmp/tidb-community-toolkit-v7.1.0-linux-amd64/bin ] ; then mkdir /tmp/tidb-community-toolkit-v7.1.0-linux-amd64/bin ; fi

clean:
	@if [ -f /tmp/tidb-community-toolkit-v7.1.0-linux-amd64.tar.gz ] ; then rm -rf /tmp/tidb-community-toolkit-v7.1.0-linux-amd64.tar.gz ; fi
	@if [ -d /tmp/tidb-community-toolkit-v7.1.0-linux-amd64 ] ; then rm -rf /tmp/tidb-community-toolkit-v7.1.0-linux-amd64 ; fi

# layer mysqldump
# layer python-base
# lambda
