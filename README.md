# Custom kubernetes controller

This is a very simple example on how to create a Kubernetes controller that automatically create a service and ingress controller when a deployment is created.

### Usage
You have to apply the deployment.yml which will create a deployment call `custom-controller` in the `kube-system` namespace, everytime you create a deployment a service and a ingress will be created. Make sure you have a ingress controller up and running.
The ingress host will be `servicename`.`domain.io` feel free to change it to your own domain. The ingress and service resource will have the same name as the deployment.

Once you delete the deployment, the service and ingress will be automatically deleted.

```
% kubectl create deployment nginx --image nginx
deployment.apps/nginx created

$ kubectl logs deployment/custom-controller -n kube-system 
Adding deployment
Creating service named nginx

$ kubectl get svc
NAME                            TYPE           CLUSTER-IP     EXTERNAL-IP     PORT(S)                      AGE
nginx                           ClusterIP      10.3.247.100   <none>          80/TCP                       48s

$ kubectl get ingress
NAME    CLASS    HOSTS             ADDRESS         PORTS   AGE
nginx   <none>   nginx.domain.io   34.79.195.242   80      77s

$ kubectl delete deployment nginx
deployment.apps "nginx" deleted

$ kubectl logs deployment/custom-controller -n kube-system 
Deleting deployment named, nginx
Deleting service nginx
Deleting ingress nginx
```

### To be improved
- Automatically collect the deployment port/name, for now it's listening only to port 80;
- Automatically recreate svc and ingress resources if it's deleted, logic still not implemented;
- Refer to annotation or owner to delete resources;