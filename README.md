# Dash controller

Dash controller is responsible to manage lifecycle of DashApplication objects.

## Local Kubernets

You can spin up kubernetes cluster using kind.
The following script deploy also load balancer and ingress controller.

```bash
$ example/kind/run-kind.sh
```

## Installation

Install CRD: 
```bash
kubectl create -f config/crd/bases
```

Now you can deploy the controller:

```bash
kubectl create -f resources/
```

Go to `example` directory to deploy your first dash application
```bash
kubectl create -f example/dash_picsum.yaml
```


```yaml
apiVersion: dash.plural.sh/v1alpha1
kind: DashApplication
metadata:
  name: picsum
  namespace: default
spec:
  image: "zreigz/dash-picsum:0.1.0"
  containerPort: 8050
  replicas: 1
```

The controller will create Deployment and Service with the DashApplication name: `picsum`
