# Create a Heketi service in OpenShift
> NOTE: This template file places the database in an _EmptyDir_ volume.  You will need to adjust accordingly if you would like the database to be on reliable persistent storage.

* Register template with OpenShift

```
oc create -f heketi.json
```

* Note the number of parameters which need to be set.  Currently only _NAME_
  needs to be set.

```
oc process --parameters heketi
```

* Deploy a Heketi service

```
oc process heketi -v NAME=myheketiservice | oc create -f -
```

* Note service

```
oc status
```

* Send a _hello_ command to service

```
curl http://<ip of service>:<port>/hello
```

* For example

```
$ oc create -f heketi-template.json 
template "heketi" created

$ oc process heketi -v NAME=myheketi | oc create -f -
service "myheketi" created
imagestream "myheketi" created
deploymentconfig "myheketi" created

$ oc status
In project default on server https://localhost:8443

svc/docker-registry - 172.30.109.242:5000
  dc/docker-registry deploys docker.io/openshift/origin-docker-registry:v1.1.4 
    deployment #1 deployed 5 days ago - 1 pod

svc/kubernetes - 172.30.0.1 ports 443, 53, 53

svc/myheketi - 172.30.72.141:8080
  dc/myheketi deploys istag/myheketi:latest 
    deployment #1 pending 3 seconds ago

View details with 'oc describe <resource>/<name>' or list everything with 'oc get all'.

$ oc get pods
NAME                      READY     STATUS    RESTARTS   AGE
myheketi-1-h1tyy          1/1       Running   0          1m

$ curl http://172.30.72.141:8080/hello
HelloWorld from GlusterFS Application
```



