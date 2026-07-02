from django.shortcuts import render, HttpResponse, redirect, reverse
from django.http import StreamingHttpResponse
from .models import Machine, VMSession, Notification
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

                    # if a service port (for exposing) has been specified, add it                    
                    if s_port > 0:
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

        # container spec -> pod templ spec -> deployment spec -> deployment

        container = client.V1Container(
            name=f"{name}-container",
            image=image,
            ports=container_ports if container_ports else None,
            env=env if env else None,
            resources=resources
        )

        template = client.V1PodTemplateSpec(
            metadata=client.V1ObjectMeta(labels={"app": name}),
            spec=client.V1PodSpec(containers=[container])
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
            kube_app_api.create_namespaced_deployment(namespace=namespace, body=deployment)
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
                kube_core_api.create_namespaced_service(namespace=namespace, body=service)
            except ApiException as e:
                return HttpResponse(f"error creating service: {str(e)}", status=503) # imagine getting this far just to have the service creation fail

    return render(request, 'deploy.html')

@login_required
def deployment_detail(request, namespace, deployment_name):
    try:
        # get the deployment
        deployment = kube_app_api.read_namespaced_deployment(name=deployment_name, namespace=namespace)
        
        # get the metadata stuff (like we did up in index)
        desired_replicas = deployment.spec.replicas or 0
        ready_replicas = deployment.status.ready_replicas or 0
        updated_replicas = deployment.status.updated_replicas or 0
        available_replicas = deployment.status.available_replicas or 0
        
        deploy_data = {
            "name": deployment.metadata.name,
            "namespace": deployment.metadata.namespace,
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
                    "image": c.image,
                    "ports": [p.container_port for p in c.ports] if c.ports else [],
                    "env": env_map,
                    "resources": {
                        "requests": reqs,
                        "limits": limits
                    }
                })
        
        # pull the service if it exists
        service = None
        try:
            service = kube_core_api.read_namespaced_service(name=f"{deployment_name.split('-')[0]}-service", namespace=namespace)
            service_ports = [{"name": p.name, "port": p.port, "target_port": p.target_port} for p in service.spec.ports] if service and service.spec else []
        except ApiException as e:
            print("error fetching service for deployment: " + str(e))
            service_ports = []

        # match deployments to their underlying runtime replicas
        # probably not a great way to do it bc u naming stuff could be messed up
        selector_labels = deployment.spec.selector.match_labels
        pod_list_ctx = []
        
        if selector_labels:
            # Convert label dict to standard comma-separated selector string (e.g. "app=my-app")
            selector_str = ",".join([f"{k}={v}" for k, v in selector_labels.items()])
            
            # kube_core_api should be an instance of client.CoreV1Api()
            managed_pods = kube_core_api.list_namespaced_pod(namespace=namespace, label_selector=selector_str)
            
            for pod in managed_pods.items:
                pod_list_ctx.append({
                    "name": pod.metadata.name,
                    "status": pod.status.phase,
                    "ip": pod.status.pod_ip,
                    "node": pod.spec.node_name,
                    "restarts": sum([c.restart_count for c in pod.status.container_statuses]) if pod.status.container_statuses else 0,
                    "created_at": pod.metadata.creation_timestamp
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
        log_stream = kube_core_api.read_namespaced_pod_log(
            name=pod_name,
            namespace=namespace,
            follow=True,                # keep conn open
            tail_lines=100,             # pre-populate with the last 100 log item (possibly have this configure?)
            _preload_content=False
        )

        for chunk in log_stream.stream(amt=1024):
            yield chunk

    return StreamingHttpResponse(log_generator(), content_type="text/plain")