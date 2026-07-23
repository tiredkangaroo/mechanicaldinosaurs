from django.shortcuts import render, HttpResponse, redirect, reverse
from django.http import StreamingHttpResponse
from .models import Machine, VMSession, Deployment
import requests, math, uuid, socket
from dateutil import parser 
from django.contrib.auth.decorators import login_required
from django.contrib.auth import authenticate, login, logout
from passkeys.models import UserPasskey
from os import environ, path
from kubernetes import client, config
from kubernetes.client.rest import ApiException
from django.conf import settings
import resend
from django.views.decorators.clickjacking import xframe_options_exempt
from . import k3

def deploy(request):
    if request.method == 'POST':
        name = request.POST.get('name').lower().strip()
        image = request.POST.get('image')
        
        # resource stuff
        cpu_resource_request = request.POST.get('cpuResourceRequest')
        memory_resource_request = request.POST.get('memoryResourceRequest')
        cpu_resource_limit = request.POST.get('cpuResourceLimit')
        memory_resource_limit = request.POST.get('memoryResourceLimit')

        replicas = int(request.POST.get('replicas', 1))
        namespace = request.POST.get('namespace', 'default')
        env_vars_raw = request.POST.get('env_vars') # format: k1=v1,k2=v2

        # we have to create a service alongside the deployment if there's any ports to expose
        expose_externally = request.POST.get('expose_externally') == 'true'
        ports_raw = request.POST.get('ports') # map of containerPort:hostPort or containerPort:0 if not to be exposed but should still be marked to kube
        image_pull_policy = request.POST.get('image_pull_policy')

        # storage
        use_persistent_storage = request.POST.get('use_persistent_storage') == 'true'
        storage_size = request.POST.get('storage_size')
        mount_path = request.POST.get('storage_mount_path')

        env = []
        if env_vars_raw: # i probably don't need to check this but just in case
            for item in env_vars_raw.split(','):
                if '=' in item:
                    k, v = item.split('=', 1)
                    env.append(client.V1EnvVar(name=k.strip(), value=v.strip()))
                else:
                    return HttpResponse(f"invalid env var format: {item}", status=400) # shouldn't happen if frontend works correctly
        
        container_ports = []
        service_ports = []

        if ports_raw: # again probably don't need to check this lol
            for mapping in ports_raw.split(','):
                if ':' in mapping:
                    c_port, s_port = mapping.split(':')
                    c_port, s_port = int(c_port.strip()), int(s_port.strip())

                    # mark the container port on the container spec 
                    container_ports.append(client.V1ContainerPort(container_port=c_port))

                    service_ports.append(client.V1ServicePort(
                        name=f"port-{c_port}", # name for the port will be the container port it is mapped to
                        port=s_port,         # Port exposed on the Service network
                        target_port=c_port   # Target port inside the container
                    ))

        # resource requests and limits
        resources = client.V1ResourceRequirements(
            requests={"cpu": cpu_resource_request, "memory": memory_resource_request},
            limits={"cpu": cpu_resource_limit, "memory": memory_resource_limit}
        )

        volume_mounts = []
        pod_volumes = []
        if use_persistent_storage:
            pvc_name = f"{name}-pvc" # persistent volume claim
            pvc = client.V1PersistentVolumeClaim(
                metadata=client.V1ObjectMeta(name=pvc_name),
                spec=client.V1PersistentVolumeClaimSpec(
                    access_modes=["ReadWriteOnce"],
                    resources=client.V1VolumeResourceRequirements(requests={"storage": storage_size})
                )
            )

            try:
                k3.kube_core_api.create_namespaced_persistent_volume_claim(namespace=namespace, body=pvc)
            except ApiException as e:
                # we could probably handle specific erorrs like "already exists" or smth
                if not e.status == 409: # note: check if this works;
                    return HttpResponse(f"error creating persistent volume claim: {str(e)}", status=503)
        
            volume_mounts.append(client.V1VolumeMount(
                name="persistent-volume-storage",
                mount_path=mount_path
            ))
            pod_volumes.append(client.V1Volume(
                name="persistent-volume-storage",
                persistent_volume_claim=client.V1PersistentVolumeClaimVolumeSource(claim_name=pvc_name)
            ))

        # container spec -> pod templ spec -> deployment spec -> deployment

        container = client.V1Container(
            name=f"{name}-container",
            image=image,
            ports=container_ports if container_ports else None,
            env=env if env else None,
            resources=resources,
            image_pull_policy=image_pull_policy,
            volume_mounts=volume_mounts if volume_mounts else None # volumes :p
        )

        template = client.V1PodTemplateSpec(
            metadata=client.V1ObjectMeta(labels={"app": name}),
            spec=client.V1PodSpec(
                containers=[container],
                volumes=pod_volumes if pod_volumes else None
            ),
        )
        
        deployment_spec = client.V1DeploymentSpec(
            replicas=replicas,
            selector=client.V1LabelSelector(match_labels={"app": name}),
            template=template
        )
        
        deployment = client.V1Deployment(
            api_version="apps/v1",
            kind="Deployment",
            metadata=client.V1ObjectMeta(name=f"{name}-deployment"),
            spec=deployment_spec
        )

        # boom wow a deployment
        try:
            k3.kube_app_api.create_namespaced_deployment(namespace=namespace, body=deployment)
        except ApiException as e:
            return HttpResponse(f"error creating deployment: {str(e)}", status=503)

        # if we have service ports, create a service
        if service_ports:
            service_spec = client.V1ServiceSpec(
                selector={"app": name},
                ports=service_ports,
                type="LoadBalancer" if expose_externally else "ClusterIP"
            )

            service = client.V1Service(
                api_version="v1",
                kind="Service",
                metadata=client.V1ObjectMeta(name=f"{name}-service"),
                spec=service_spec
            )

            try:
                k3.kube_core_api.create_namespaced_service(namespace=namespace, body=service)
            except ApiException as e:
                return HttpResponse(f"error creating service: {str(e)}", status=503) # imagine getting this far just to have the service creation fail

            try:
                Deployment.objects.create(name=f"{name}-deployment", replicas_desired=replicas, service_name=f"{name}-service")
            except Exception as e:
                print(f"error creating deployment record in db: {str(e)}")
        return redirect('deployment_detail', namespace=namespace, deployment_name=f"{name}-deployment")

    return render(request, 'deploy.html')

@login_required
def deployment_detail(request, namespace, deployment_name):
    try:
        # get the deployment
        deployment = k3.kube_app_api.read_namespaced_deployment(name=deployment_name, namespace=namespace)
        
        # get the metadata stuff (like we did up in index)
        desired_replicas = deployment.spec.replicas or 0
        ready_replicas = deployment.status.ready_replicas or 0
        updated_replicas = deployment.status.updated_replicas or 0
        available_replicas = deployment.status.available_replicas or 0

        status = "Unknown"
        if ready_replicas == desired_replicas:
            if ready_replicas == 0:
                status = "Stopped"
            else:   
                status = "Healthy"
        elif ready_replicas == 0: # oop
            status = "Critical"
        else:
            status = "Scaling"
        
        deploy_data = {
            "name": deployment.metadata.name,
            "namespace": deployment.metadata.namespace,
            "status": status,
            "strategy": deployment.spec.strategy.type if deployment.spec.strategy else "RollingUpdate",
            "replicas_desired": desired_replicas,
            "replicas_ready": ready_replicas,
            "replicas_updated": updated_replicas,
            "replicas_available": available_replicas,
            "labels": deployment.metadata.labels or {},
            "created_at": deployment.metadata.creation_timestamp,
        }

        # get containers (this is where we can pull out env vars, resource requests/limits, etc.)
        containers = []
        if deployment.spec.template.spec.containers:
            for c in deployment.spec.template.spec.containers:
                # env stuff
                env_map = {}
                if c.env:
                    for env_var in c.env:
                        if env_var.value is not None:
                            env_map[env_var.name] = env_var.value
                        elif env_var.value_from:
                            env_map[env_var.name] = "[Value From Source]"

                # limits / requests
                reqs = getattr(c.resources, 'requests', {}) or {}
                limits = getattr(c.resources, 'limits', {}) or {}

                containers.append({
                    "name": c.name,
                    "command": c.command,
                    "image": c.image,
                    "ports": c.ports,
                    "env_from": c.env_from,
                    "env": env_map,
                    "image_pull_policy": c.image_pull_policy,
                    "restart_policy": c.restart_policy,
                    "resources": {
                        "requests": reqs,
                        "limits": limits
                    },
                    "volume_mounts": c.volume_mounts
                })
        
        # pull the service if it exists

        service_ports = []
        try:
            db_deployment = Deployment.objects.get(name=deployment_name)
            try:
                service = k3.kube_core_api.read_namespaced_service(name=f"{db_deployment.service_name}", namespace=namespace)
                print(f"service ports: {service.spec.ports}")
                service_ports = [{"name": p.name, "port": p.port, "target_port": p.target_port} for p in service.spec.ports] if service and service.spec else []
            except ApiException as e:
                print("error fetching service for deployment: " + str(e))
                service_ports = []
        except Deployment.DoesNotExist:
            pass

        # match deployments to their underlying runtime replicas
        # probably not a great way to do it bc u naming stuff could be messed up
        selector_labels = deployment.spec.selector.match_labels
        pod_list_ctx = []
        
        if selector_labels:
            # Convert label dict to standard comma-separated selector string (e.g. "app=my-app")
            selector_str = ",".join([f"{k}={v}" for k, v in selector_labels.items()])
            
            # k3.kube_core_api should be an instance of client.CoreV1Api()
            managed_pods = k3.kube_core_api.list_namespaced_pod(namespace=namespace, label_selector=selector_str)
            
            for pod in managed_pods.items:
                pod_list_ctx.append({
                    "name": pod.metadata.name,
                    "status": pod.status.phase,
                    "ip": pod.status.pod_ip,
                    "node": pod.spec.node_name,
                    "restarts": sum([c.restart_count for c in pod.status.container_statuses]) if pod.status.container_statuses else 0,
                    "created_at": pod.metadata.creation_timestamp,
                    "volumes": pod.spec.volumes
                })

        context = {
            "deployment": deploy_data,
            "containers": containers,
            "service_ports": service_ports,
            "pods": pod_list_ctx
        }
        
        return render(request, 'deployment_detail.html', context)
    except ApiException as e:
        return HttpResponse(f"error fetching deployment details: {str(e)}", status=503)

@xframe_options_exempt
def pod_logs(request, namespace, pod_name):
    def log_generator():
        try:
            log_stream = k3.kube_core_api.read_namespaced_pod_log(
                name=pod_name,
                namespace=namespace,
                follow=True,                # keep conn open
                tail_lines=100,             # pre-populate with the last 100 log item (possibly have this configure?)
                _preload_content=False
            )

            for chunk in log_stream.stream(amt=1024):
                yield chunk
        except Exception as e:
            yield f"error: {str(e)}"

    return StreamingHttpResponse(log_generator(), content_type="text/plain")

def stop_deployment(request, namespace, deployment_name):
    try:
        # see if deployment exists in db. if it doesn't, create it to store the replicas desired value as the current one
        try:
            Deployment.objects.get(name=deployment_name)
        except Deployment.DoesNotExist:
            # fetch the deployment to get the current replicas desired value
            deployment = k3.kube_app_api.read_namespaced_deployment(name=deployment_name, namespace=namespace)
            current_replicas = deployment.spec.replicas or 0
            Deployment.objects.create(name=deployment_name, replicas_desired=current_replicas, service_name=None) # service name must be None bc we don't know it and it may not exist

        # scale down the deployment to 0 replicas
        body = client.V1Scale(
            spec=client.V1ScaleSpec(replicas=0)
        )
        k3.kube_app_api.patch_namespaced_deployment_scale(name=deployment_name, namespace=namespace, body=body)
        return redirect('deployment_detail', namespace=namespace, deployment_name=deployment_name)
    except ApiException as e:
        return HttpResponse(f"error stopping deployment: {str(e)}", status=503)

def start_deployment(request, namespace, deployment_name):
    desired_replicas = 1 # default to 1 if we can't find the record in db
    try:
        # fetch the deployment record from db to get the desired replicas value
        deployment_record = Deployment.objects.get(name=deployment_name)
        desired_replicas = deployment_record.replicas_desired
    except:
        pass

    try:
        # scale up the deployment to the desired replicas
        body = client.V1Scale(
            spec=client.V1ScaleSpec(replicas=desired_replicas)
        )
        k3.kube_app_api.patch_namespaced_deployment_scale(name=deployment_name, namespace=namespace, body=body)
        return redirect('deployment_detail', namespace=namespace, deployment_name=deployment_name)
    except ApiException as e:
        return HttpResponse(f"error starting deployment: {str(e)}", status=503)

def delete_deployment(request, namespace, deployment_name):
    deployment = None
    try:
        deployment = Deployment.objects.get(name=deployment_name)
    except Deployment.DoesNotExist:
        pass

    try:
        # delete the deployment
        k3.kube_app_api.delete_namespaced_deployment(name=deployment_name, namespace=namespace)
        
        if deployment and deployment.service_name:
            try:
                k3.kube_core_api.delete_namespaced_service(name=f"{deployment.service_name}", namespace=namespace)
            except ApiException as e:
                print(f"error deleting service for deployment: {str(e)}")
        
        return redirect('index')
    except ApiException as e:
        return HttpResponse(f"error deleting deployment: {str(e)}", status=503)
    
    try:
        deployment.delete()
    except Exception as e:
        print(f"error deleting deployment record from db: {str(e)}")
    
    return redirect('index')