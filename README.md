```
$ ./coreup -help
Usage of ./coreup:
  -action="run": run, terminate, list
  -channel="alpha": CoreOS channel to use
  -cloud-config="": local file, usually ./cloud-config.yml
  -num=1: number of instances to launch like this
  -project="coreup-<user>": name for the group of servers in the same project
  -provider="ec2": cloud or provider to launch instance in
  -region="us-west-2": region to launch instance in
  -size="m1.medium": size of instance
```

Only supports ec2 for now. 

```
go get github.com/polvi/coreup
... place a cloud-config.yml in your working directory ...
coreup -num 3
```
