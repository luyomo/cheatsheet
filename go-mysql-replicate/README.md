./bin/dm-master -L debug -log-file=/mnt/dm/dm-master.log -master-addr=10.0.0.4:8261   -data-dir=/mnt/dm/data &

```
$ ./bin/dm-master -config=./dm/master/dm-master.toml & 
```


```
$ ./bin/dm-worker -config dm/worker/dm-worker.toml  -name dmworker01 &
#log configuration
log-level = "debug"
log-file = "/mnt/dm/dm-worker.log"

#dm-worker listen address
worker-addr = ":8262"
advertise-addr = "10.0.0.4:8262"
join = "10.0.0.4:8261"

relay-dir = "/tmp/relay"
```

```
$ mkdir -p /home/azureuser/workspace/github.com/go-mysql-org 
$ ln -s /home/azureuser/workspace/go-mysql /home/azureuser/workspace/github.com/go-mysql-org/go-mysql@v1.9.1
$ go tool pprof --source_path=/home/azureuser/workspace \
    /home/azureuser/workspace/tiflow/bin/dm-worker \
    /mnt/repli/dm_oom.pb
$ list decodeImage
```

```
export PPROF_SOURCE_PATH="/home/azureuser/workspace"
go tool pprof /home/.../dm-worker /mnt/repli/dm_oom.pb
```


```
select max(id) from target_tmp where id <  13465612;
select count(*) from target_tmp where id between  13465612 and 23531018;

select count(*) from web_send_record_tmp where id < 23531018;
select count(*) from web_send_record_tmp where id between 23531018 and 43531018;
select count(*) from web_send_record_tmp where id > 43531018;

```

tiup dm patch dmlocalhost /tmp/dm-worker-v8.5.0-hotfix-linux-amd64.tar.gz -R dm-worker --overwrite
