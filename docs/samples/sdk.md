# Interface with Python Client [Sdk](https://kserve.github.io/website/0.10/sdk_docs/sdk_doc/#getting-started)

## Methods
- set_credentials
- create
- get
- patch
- replace
- delete
- wait_isvc_ready
- is_isvc_ready

## Examples
- Assuming that you have configured the environment with the right resources

#### Instantiate client 
```
kserve = KServeClient()
```


#### create
```
kserve.create(isvc)
```


#### get
```
kserve.get(service_name, namespace=namespace)
```


#### replace
```
kserve.replace(service_name, isvc)
```


#### delete
```
kserve.delete(service_name, namespace=namespace)
```


#### wait_isvc_ready
```
kserve.wait_isvc_ready(service_name, namespace=namespace)
```


#### is_isvc_ready
```
kserve.is_isvc_ready(service_name, namespace=namespace)
```