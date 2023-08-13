version = v7.1.0
packageName = tidb-community-toolkit-$(version)-linux-amd64
dumplingPackageName = dumpling-$(version)-linux-amd64
s3uri = s3://jay-data/lambda/layers/

build: clean mysqldump python-base lambda cf

dumpling: base
	@if [ ! -f /tmp/$(packageName)/bin/dumpling ] ; then tar xvf /tmp/$(packageName)/$(dumplingPackageName).tar.gz -C /tmp/$(packageName)/bin ; fi
	@if [ ! -f /tmp/$(packageName)/$(dumplingPackageName).zip ] ; then cd /tmp/$(packageName); zip -r $(dumplingPackageName).zip bin ; fi
	aws s3 cp /tmp/$(packageName)/$(dumplingPackageName).zip $(s3uri)

base:
	@if [ ! -f /tmp/$(packageName).tar.gz ] ; then wget https://download.pingcap.org/$(packageName).tar.gz -P /tmp ; fi
	@if [ ! -d /tmp/$(packageName) ] ; then tar xvf /tmp/$(packageName).tar.gz -C /tmp ; fi
	@if [ ! -d /tmp/$(packageName)/bin ] ; then mkdir /tmp/$(packageName)/bin ; fi

clean:
	@if [ -f /tmp/$(packageName).tar.gz ] ; then rm -rf /tmp/$(packageName).tar.gz ; fi
	@if [ -d /tmp/$(packageName) ] ; then rm -rf /tmp/$(packageName) ; fi

# layer mysqldump
# layer python-base
# lambda
